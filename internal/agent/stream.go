package agent

import (
	"context"
	"errors"
	"fmt"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
	"goose-go/internal/session"
)

func (a *Agent) Reply(ctx context.Context, sessionID string, userText string) (Result, error) {
	stream, err := a.ReplyStream(ctx, sessionID, userText)
	if err != nil {
		return Result{}, err
	}
	return consumeTerminalResult(stream)
}

func (a *Agent) ResolveApproval(ctx context.Context, sessionID string, decision ApprovalDecision) (Result, error) {
	stream, err := a.ResolveApprovalStream(ctx, sessionID, decision)
	if err != nil {
		return Result{}, err
	}
	return consumeTerminalResult(stream)
}

func (a *Agent) ReplyStream(ctx context.Context, sessionID string, userText string) (<-chan Event, error) {
	return a.stream(ctx, sessionID, func(ctx context.Context, emit func(Event)) (Result, error) {
		return a.reply(ctx, sessionID, userText, emit)
	})
}

func (a *Agent) ResolveApprovalStream(ctx context.Context, sessionID string, decision ApprovalDecision) (<-chan Event, error) {
	return a.stream(ctx, sessionID, func(ctx context.Context, emit func(Event)) (Result, error) {
		return a.resolveApproval(ctx, sessionID, decision, emit)
	})
}

func (a *Agent) stream(ctx context.Context, sessionID string, run func(context.Context, func(Event)) (Result, error)) (<-chan Event, error) {
	events := make(chan Event, 32)
	go func() {
		defer close(events)

		result, err := run(ctx, func(event Event) {
			select {
			case events <- event:
			case <-ctx.Done():
			}
		})

		terminalEvent := Event{
			SessionID: sessionID,
			Result:    resultOrNil(result),
		}
		switch {
		case err == nil:
			terminalEvent.Type = EventTypeRunCompleted
		case errors.Is(err, context.Canceled):
			terminalEvent.Type = EventTypeRunInterrupted
			terminalEvent.Err = err
		default:
			terminalEvent.Type = EventTypeRunFailed
			terminalEvent.Err = err
		}

		events <- terminalEvent
	}()

	return events, nil
}

func consumeTerminalResult(stream <-chan Event) (Result, error) {
	var (
		result  Result
		runErr  error
		haveEnd bool
	)
	for event := range stream {
		switch event.Type {
		case EventTypeRunCompleted:
			haveEnd = true
			if event.Result != nil {
				result = *event.Result
			}
		case EventTypeRunInterrupted:
			haveEnd = true
			runErr = context.Canceled
			if event.Result != nil {
				result = *event.Result
			}
		case EventTypeRunFailed:
			haveEnd = true
			runErr = event.Err
			if event.Result != nil {
				result = *event.Result
			}
		}
	}
	if !haveEnd {
		return Result{}, errors.New("agent stream ended without terminal event")
	}
	return result, runErr
}

func (a *Agent) reply(ctx context.Context, sessionID string, userText string, emit func(Event)) (Result, error) {
	if userText == "" {
		return Result{}, errors.New("user text is required")
	}

	emit(Event{Type: EventTypeRunStarted, SessionID: sessionID})

	userMessage := conversation.NewMessage(conversation.RoleUser, conversation.Text(userText))
	record, err := a.store.AddMessage(ctx, sessionID, userMessage)
	if err != nil {
		return Result{}, fmt.Errorf("append user message: %w", err)
	}
	emit(Event{Type: EventTypeUserMessagePersisted, SessionID: sessionID, Message: &userMessage})

	return a.runTurns(ctx, record, 1, emit)
}

func (a *Agent) runTurns(ctx context.Context, record session.Session, startTurn int, emit func(Event)) (Result, error) {
	for turn := startTurn; turn <= a.config.MaxTurns; turn++ {
		emit(Event{Type: EventTypeTurnStarted, SessionID: record.ID, Turn: turn})

		activeMessages, err := a.prepareActiveMessages(ctx, record, turn, emit)
		if err != nil {
			return Result{}, err
		}

		recoveredFromOverflow := false
		var assistantMessage conversation.Message
		for {
			assistantMessage, err = a.runProviderTurn(ctx, record, activeMessages, turn, emit)
			if err == nil {
				break
			}
			if !isContextOverflowError(err) || recoveredFromOverflow {
				return Result{}, err
			}

			activeMessages, err = a.compactForReason(ctx, record, turn, session.CompactionTriggerOverflow, emit)
			if err != nil {
				return Result{}, err
			}
			recoveredFromOverflow = true
		}
		emit(Event{Type: EventTypeAssistantMessageComplete, SessionID: record.ID, Turn: turn, Message: &assistantMessage})

		record, err = a.store.AddMessage(ctx, record.ID, assistantMessage)
		if err != nil {
			return Result{}, fmt.Errorf("append assistant message: %w", err)
		}
		emit(Event{Type: EventTypeAssistantMessagePersisted, SessionID: record.ID, Turn: turn, Message: &assistantMessage})

		toolCalls := extractToolCalls(assistantMessage)
		if len(toolCalls) == 0 {
			return Result{Status: StatusCompleted, Session: record, Turns: turn}, nil
		}

		var pending *Result
		record, pending, err = a.processToolCalls(ctx, record, turn, toolCalls, nil, emit)
		if err != nil {
			return Result{}, err
		}
		if pending != nil {
			return *pending, nil
		}
	}

	return Result{Status: StatusCompleted, Session: record, Turns: a.config.MaxTurns}, ErrMaxTurnsExceeded
}

func (a *Agent) runProviderTurn(ctx context.Context, record session.Session, messages []conversation.Message, turn int, emit func(Event)) (conversation.Message, error) {
	stream, err := a.provider.Stream(ctx, provider.Request{
		SessionID:    record.ID,
		SystemPrompt: a.config.SystemPrompt,
		Messages:     messages,
		Tools:        toolDefinitions(a.tools.Definitions()),
		Model:        a.config.Model,
	})
	if err != nil {
		return conversation.Message{}, fmt.Errorf("start provider turn: %w", err)
	}

	var finalMessage *conversation.Message
	for event := range stream {
		switch event.Type {
		case provider.EventTypeTextDelta:
			emit(Event{Type: EventTypeProviderTextDelta, SessionID: record.ID, Turn: turn, Delta: event.Delta})
		case provider.EventTypeMessageComplete:
			finalMessage = event.Message
		case provider.EventTypeError:
			return conversation.Message{}, fmt.Errorf("provider turn failed: %w", event.Err)
		}
	}

	if finalMessage == nil {
		return conversation.Message{}, errors.New("provider turn did not produce a final assistant message")
	}

	return *finalMessage, nil
}
