package archcheck

var Rules = []Rule{
	{
		FromPrefix: "goose-go/internal/provider",
		ToPrefix:   "goose-go/internal/storage/",
		Message:    "providers must not depend on storage implementations",
	},
	{
		FromPrefix: "goose-go/internal/provider",
		ToPrefix:   "goose-go/cmd/",
		Message:    "providers must not depend on CLI entrypoints",
	},
	{
		FromPrefix: "goose-go/internal/session",
		ToPrefix:   "goose-go/internal/storage/",
		Message:    "session contracts must not depend on storage implementations",
	},
	{
		FromPrefix: "goose-go/internal/session",
		ToPrefix:   "goose-go/internal/provider",
		Message:    "session contracts must not depend on providers",
	},
	{
		FromPrefix: "goose-go/internal/tools",
		ToPrefix:   "goose-go/internal/provider/",
		Message:    "tools must not depend on provider implementations",
	},
	{
		FromPrefix: "goose-go/internal/auth/",
		ToPrefix:   "goose-go/internal/agent",
		Message:    "auth packages must not depend on agent runtime packages",
	},
	{
		FromPrefix: "goose-go/internal/auth/",
		ToPrefix:   "goose-go/internal/app",
		Message:    "auth packages must not depend on app-layer packages",
	},
	{
		FromPrefix: "goose-go/internal/auth/",
		ToPrefix:   "goose-go/internal/provider",
		Message:    "auth packages must not depend on provider packages",
	},
	{
		FromPrefix: "goose-go/internal/auth/",
		ToPrefix:   "goose-go/internal/session",
		Message:    "auth packages must not depend on session packages",
	},
	{
		FromPrefix: "goose-go/internal/auth/",
		ToPrefix:   "goose-go/internal/storage/",
		Message:    "auth packages must not depend on storage implementations",
	},
	{
		FromPrefix: "goose-go/internal/auth/",
		ToPrefix:   "goose-go/internal/tools",
		Message:    "auth packages must not depend on tool runtime packages",
	},
	{
		FromPrefix: "goose-go/internal/provider/openaicodex",
		ToPrefix:   "goose-go/internal/agent",
		Message:    "provider implementations must not depend on agent runtime packages",
	},
	{
		FromPrefix: "goose-go/internal/provider/openaicodex",
		ToPrefix:   "goose-go/internal/app",
		Message:    "provider implementations must not depend on app-layer packages",
	},
	{
		FromPrefix: "goose-go/internal/provider/openaicodex",
		ToPrefix:   "goose-go/internal/session",
		Message:    "provider implementations must not depend on session packages",
	},
	{
		FromPrefix: "goose-go/internal/provider/openaicodex",
		ToPrefix:   "goose-go/internal/storage/",
		Message:    "provider implementations must not depend on storage implementations",
	},
	{
		FromPrefix: "goose-go/internal/provider/openaicodex",
		ToPrefix:   "goose-go/internal/tools",
		Message:    "provider implementations must not depend on tool runtime packages",
	},
	{
		FromPrefix: "goose-go/internal/storage/",
		ToPrefix:   "goose-go/internal/agent",
		Message:    "storage implementations must not depend on agent runtime packages",
	},
	{
		FromPrefix: "goose-go/internal/storage/",
		ToPrefix:   "goose-go/internal/app",
		Message:    "storage implementations must not depend on app-layer packages",
	},
	{
		FromPrefix: "goose-go/internal/storage/",
		ToPrefix:   "goose-go/internal/auth/",
		Message:    "storage implementations must not depend on auth packages",
	},
	{
		FromPrefix: "goose-go/internal/storage/",
		ToPrefix:   "goose-go/internal/provider",
		Message:    "storage implementations must not depend on provider packages",
	},
	{
		FromPrefix: "goose-go/internal/storage/",
		ToPrefix:   "goose-go/internal/tools",
		Message:    "storage implementations must not depend on tool runtime packages",
	},
	{
		FromPrefix: "goose-go/internal/evals",
		ToPrefix:   "goose-go/internal/app",
		Message:    "eval harness should test runtime boundaries directly instead of depending on app-layer composition",
	},
}
