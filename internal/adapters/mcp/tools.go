package mcp

import (
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerTools() {
	kbIDsItemSchema := map[string]any{
		"oneOf": []any{
			map[string]any{
				"type":        "string",
				"description": `Bare id ("notes") or "scope:id" ("project:notes").`,
			},
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scope": map[string]any{"type": "string", "enum": []string{"project", "global"}},
					"id":    map[string]any{"type": "string"},
				},
				"required":             []string{"id"},
				"additionalProperties": false,
			},
		},
	}
	scopeProperty := mcpgo.WithString(
		"scope",
		mcpgo.Description("Knowledge base scope. Defaults to project when running in a project directory, otherwise global."),
		mcpgo.Enum("project", "global"),
	)
	searchTool := mcpgo.NewTool(
		"search_knowledge",
		mcpgo.WithDescription("按语义/词法/混合模式检索 knowledge 类型知识库中的条目（Q&A、项目决策、调试配方、参考资料）。返回带 snippet 的命中结果；用 get_knowledge_item 拉完整正文。规范库通过 list_knowledge_bases 识别，并用 list_knowledge_items 全量读取。"),
		mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("搜索查询词。")),
		mcpgo.WithArray(
			"kb_ids",
			mcpgo.Description(`可选 KB 引用列表。每个元素可以是裸 id（"notes"）、"scope:id" 字符串（"project:notes"）或对象 {"scope":"project","id":"notes"}。`),
			mcpgo.Items(kbIDsItemSchema),
		),
		scopeProperty,
		mcpgo.WithNumber("limit", mcpgo.Description("最多返回条数。"), mcpgo.DefaultNumber(10)),
		mcpgo.WithString("search_mode", mcpgo.Description("搜索模式。"), mcpgo.Enum("auto", "lexical", "semantic", "hybrid"), mcpgo.DefaultString("auto")),
		mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	getTool := mcpgo.NewTool(
		"get_knowledge_item",
		mcpgo.WithDescription("Fetch the full content and metadata of a single knowledge item by KB id + item id. Use after list_knowledge_items or list_knowledge_bases surfaces a promising hit and you need the complete text — to answer a user's question accurately, cite in a technical design/spec, or apply to code. Cheap and read-only; prefer fetching full content over deciding from a title alone."),
		scopeProperty,
		mcpgo.WithString("kb_id", mcpgo.Required(), mcpgo.Description("Knowledge base ID.")),
		mcpgo.WithString("item_id", mcpgo.Required(), mcpgo.Description("Knowledge item ID.")),
		mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	listItemsTool := mcpgo.NewTool(
		"list_knowledge_items",
		mcpgo.WithDescription("Browse a knowledge base as a lightweight directory (id/title/tags, no content). Use when: (1) the user asks 'what is in KB X', (2) you need to scan a KB exhaustively before answering a question, drafting a technical plan, or writing code. After spotting a promising id, call get_knowledge_item for the full content."),
		scopeProperty,
		mcpgo.WithString("kb_id", mcpgo.Required(), mcpgo.Description("Knowledge base ID.")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of items to return. 0 means all.")),
		mcpgo.WithNumber("offset", mcpgo.Description("Number of items to skip from the start.")),
		mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	addTool := mcpgo.NewTool(
		"add_knowledge_item",
		mcpgo.WithDescription("Add a knowledge item to a knowledge base."),
		scopeProperty,
		mcpgo.WithString("kb_id", mcpgo.Required(), mcpgo.Description("Knowledge base ID.")),
		mcpgo.WithString("title", mcpgo.Required(), mcpgo.Description("Item title.")),
		mcpgo.WithString("content", mcpgo.Required(), mcpgo.Description("Item content.")),
		mcpgo.WithArray("tags", mcpgo.Description("Optional item tags."), mcpgo.WithStringItems()),
		mcpgo.WithObject("metadata", mcpgo.Description("Optional item metadata."), mcpgo.AdditionalProperties(true)),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(false),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	deleteTool := mcpgo.NewTool(
		"delete_knowledge_item",
		mcpgo.WithDescription("Delete a knowledge item from a knowledge base."),
		scopeProperty,
		mcpgo.WithString("kb_id", mcpgo.Required(), mcpgo.Description("Knowledge base ID.")),
		mcpgo.WithString("item_id", mcpgo.Required(), mcpgo.Description("Knowledge item ID.")),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(true),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	listTool := mcpgo.NewTool(
		"list_knowledge_bases",
		mcpgo.WithDescription("List all configured knowledge bases AND every item id/title/tags. CALL EARLY at the start of any non-trivial task — title/tag scans surface entries that might otherwise be missed. Cheap, read-only; one upfront call beats guessing kb_ids. Use get_knowledge_item for full content."),
		mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	createKBTool := mcpgo.NewTool(
		"create_knowledge_base",
		mcpgo.WithDescription("Create a new knowledge base. Path is required for global scope; for project scope a relative path is resolved against the project root."),
		scopeProperty,
		mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("Knowledge base ID (letters, digits, underscore, dash, dot; max 64 chars).")),
		mcpgo.WithString("type", mcpgo.Description("Knowledge base type. Defaults to knowledge."), mcpgo.Enum("knowledge", "specification")),
		mcpgo.WithString("name", mcpgo.Description("Human-readable name. Defaults to id.")),
		mcpgo.WithString("store_type", mcpgo.Required(), mcpgo.Description("Backend store type."), mcpgo.Enum("text", "sqlite")),
		mcpgo.WithString("path", mcpgo.Description("Storage path. Required for global scope; relative paths for project scope are resolved against the project root.")),
		mcpgo.WithBoolean("enabled", mcpgo.Description("Whether the knowledge base is enabled. Defaults to true.")),
		mcpgo.WithBoolean("semantic_enabled", mcpgo.Description("Enable semantic indexing for sqlite store types.")),
		mcpgo.WithArray("tags", mcpgo.Description("Optional knowledge base tags."), mcpgo.WithStringItems()),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(false),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	deleteKBTool := mcpgo.NewTool(
		"delete_knowledge_base",
		mcpgo.WithDescription("Delete a runtime-managed knowledge base. Static knowledge bases declared in config files cannot be deleted."),
		scopeProperty,
		mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("Knowledge base ID.")),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(true),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	indexTool := mcpgo.NewTool(
		"index_knowledge",
		mcpgo.WithDescription("Backfill or rebuild semantic indexes for one knowledge base or all enabled knowledge bases."),
		scopeProperty,
		mcpgo.WithString("kb_id", mcpgo.Description("Optional knowledge base ID. Omit to index all enabled knowledge bases.")),
		mcpgo.WithBoolean("rebuild", mcpgo.Description("Delete existing semantic vectors before indexing.")),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(true),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)

	gitKnowledgeAddTool := mcpgo.NewTool(
		"git_knowledge_add",
		mcpgo.WithDescription("Clone a git repository as a text knowledge base. Stores to ~/.knowledger/git-knowledge/<id>/ (global) or <project>/.knowledger/git-knowledge/<id>/ (project) and registers it. After cloning, call index_knowledge to index it."),
		scopeProperty,
		mcpgo.WithString("url", mcpgo.Required(), mcpgo.Description("Git repository URL to clone.")),
		mcpgo.WithString("id", mcpgo.Description("Knowledge base ID (derived from repository name if omitted).")),
		mcpgo.WithString("name", mcpgo.Description("Human-readable name (defaults to id).")),
		mcpgo.WithString("type", mcpgo.Description("Knowledge base type. Defaults to knowledge."), mcpgo.Enum("knowledge", "specification")),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(false),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	gitKnowledgePullTool := mcpgo.NewTool(
		"git_knowledge_pull",
		mcpgo.WithDescription("Pull latest changes for a git-knowledge knowledge base. After pulling, call index_knowledge to reindex."),
		scopeProperty,
		mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("Knowledge base ID.")),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	gitKnowledgeListTool := mcpgo.NewTool(
		"git_knowledge_list",
		mcpgo.WithDescription("List all git-knowledge knowledge bases from global (~/.knowledger/git-knowledge/) and project (.knowledger/git-knowledge/) directories. Only returns directories that are still registered as knowledge bases."),
		mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)

	s.tools = []mcpgo.Tool{searchTool, getTool, listItemsTool, addTool, deleteTool, listTool, createKBTool, deleteKBTool, indexTool, gitKnowledgeAddTool, gitKnowledgePullTool, gitKnowledgeListTool}
	s.server.AddTool(searchTool, s.handleSearchKnowledge)
	s.server.AddTool(getTool, s.handleGetKnowledgeItem)
	s.server.AddTool(listItemsTool, s.handleListKnowledgeItems)
	s.server.AddTool(addTool, s.handleAddKnowledgeItem)
	s.server.AddTool(deleteTool, s.handleDeleteKnowledgeItem)
	s.server.AddTool(listTool, s.handleListKnowledgeBases)
	s.server.AddTool(createKBTool, s.handleCreateKnowledgeBase)
	s.server.AddTool(deleteKBTool, s.handleDeleteKnowledgeBase)
	s.server.AddTool(indexTool, s.handleIndexKnowledge)
	s.server.AddTool(gitKnowledgeAddTool, s.handleGitKnowledgeAdd)
	s.server.AddTool(gitKnowledgePullTool, s.handleGitKnowledgePull)
	s.server.AddTool(gitKnowledgeListTool, s.handleGitKnowledgeList)
}
