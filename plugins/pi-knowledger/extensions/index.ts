import process from "node:process";

import { StringEnum } from "@earendil-works/pi-ai";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import {
  DEFAULT_MAX_BYTES,
  DEFAULT_MAX_LINES,
  formatSize,
  truncateHead,
} from "@earendil-works/pi-coding-agent";
import { Text } from "@earendil-works/pi-tui";
import { Type } from "typebox";

const binary = process.env.KNOWLEDGER_BIN?.trim() || "knowledger";
const configPath = process.env.KNOWLEDGER_CONFIG?.trim();
const outputLimit = `${DEFAULT_MAX_LINES} lines or ${formatSize(DEFAULT_MAX_BYTES)}`;

const Scope = StringEnum(["project", "global"] as const);
const SearchMode = StringEnum(["auto", "lexical", "semantic", "hybrid"] as const);

type RunDetails = {
  command: string;
  stderr?: string;
  truncated: boolean;
};

function baseArgs(): string[] {
  return configPath ? ["--config", configPath] : [];
}

function withScope(scope: "project" | "global" | undefined, args: string[]): string[] {
  if (!scope) return [...baseArgs(), ...args];
  const [command, ...commandArgs] = args;
  return [...baseArgs(), command, "--scope", scope, ...commandArgs];
}

function appendRepeated(args: string[], flag: string, values: readonly string[] | undefined): void {
  for (const value of values ?? []) args.push(flag, value);
}

function renderCall(label: string, detail?: string) {
  return (_args: unknown, theme: any) => {
    let text = theme.fg("toolTitle", theme.bold(label));
    if (detail) text += theme.fg("muted", ` ${detail}`);
    return new Text(text, 0, 0);
  };
}

export default function knowledgerExtension(pi: ExtensionAPI) {
  async function run(args: string[], cwd: string, signal?: AbortSignal) {
    const result = await pi.exec(binary, args, { cwd, signal, timeout: 120_000 });
    const command = [binary, ...args].join(" ");

    if (result.code !== 0) {
      const reason = result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`;
      throw new Error(`Knowledger command failed: ${reason}`);
    }

    const stdout = result.stdout.trim();
    const truncation = truncateHead(stdout || "(no output)", {
      maxLines: DEFAULT_MAX_LINES,
      maxBytes: DEFAULT_MAX_BYTES,
    });
    let text = truncation.content;
    if (truncation.truncated) {
      text += `\n\n[Knowledger output truncated to ${outputLimit}. Refine the query or list a narrower knowledge base.]`;
    }

    const details: RunDetails = {
      command,
      stderr: result.stderr.trim() || undefined,
      truncated: truncation.truncated,
    };
    return { content: [{ type: "text" as const, text }], details };
  }

  pi.registerTool({
    name: "search_knowledge",
    label: "Search Knowledge",
    description: `Search Knowledger using lexical, semantic, hybrid, or automatic retrieval. Results contain item ids and previews; use get_knowledge_item for full content. Output is limited to ${outputLimit}.`,
    promptSnippet: "Search durable project and global knowledge before relying on memory",
    promptGuidelines: [
      "Use search_knowledge when saved decisions, conventions, prior debugging results, or user-requested knowledge may affect the task.",
      "Use get_knowledge_item to read a promising search hit in full before applying it.",
    ],
    parameters: Type.Object({
      query: Type.String({ description: "Search query" }),
      kb_ids: Type.Optional(Type.Array(Type.String({ description: "Bare id or scope:id" }))),
      scope: Type.Optional(Scope),
      limit: Type.Optional(Type.Integer({ minimum: 1, maximum: 100 })),
      search_mode: Type.Optional(SearchMode),
    }),
    async execute(_id, params, signal, _update, ctx) {
      const args = ["search", "--query", params.query, "--limit", String(params.limit ?? 10)];
      if (params.search_mode) args.push("--search-mode", params.search_mode);
      appendRepeated(args, "--kb-id", params.kb_ids);
      return run(withScope(params.scope, args), ctx.cwd, signal);
    },
    renderCall(args, theme) {
      return new Text(
        theme.fg("toolTitle", theme.bold("search_knowledge ")) +
          theme.fg("accent", JSON.stringify(args.query)),
        0,
        0,
      );
    },
  });

  pi.registerTool({
    name: "get_knowledge_item",
    label: "Get Knowledge Item",
    description: `Read one complete Knowledger item by knowledge base and item id. Output is limited to ${outputLimit}.`,
    parameters: Type.Object({ kb_id: Type.String(), item_id: Type.String(), scope: Type.Optional(Scope) }),
    async execute(_id, params, signal, _update, ctx) {
      return run(withScope(params.scope, ["get", "--kb", params.kb_id, "--id", params.item_id]), ctx.cwd, signal);
    },
    renderCall(args, theme) {
      return renderCall("get_knowledge_item", `${args.kb_id}:${args.item_id}`)(args, theme);
    },
  });

  pi.registerTool({
    name: "list_knowledge_items",
    label: "List Knowledge Items",
    description: `List item ids, titles, metadata, and summaries in one knowledge base without loading full content. Output is limited to ${outputLimit}.`,
    parameters: Type.Object({ kb_id: Type.String(), scope: Type.Optional(Scope) }),
    async execute(_id, params, signal, _update, ctx) {
      return run(withScope(params.scope, ["list-items", "--kb", params.kb_id]), ctx.cwd, signal);
    },
    renderCall(args, theme) {
      return renderCall("list_knowledge_items", args.kb_id)(args, theme);
    },
  });

  pi.registerTool({
    name: "list_knowledge_bases",
    label: "List Knowledge Bases",
    description: `List configured knowledge bases and their item directories. Output is limited to ${outputLimit}.`,
    parameters: Type.Object({ scope: Type.Optional(StringEnum(["project", "global", "all"] as const)) }),
    async execute(_id, params, signal, _update, ctx) {
      const args = ["list-kbs"];
      if (params.scope) args.push("--scope-filter", params.scope);
      return run([...baseArgs(), ...args], ctx.cwd, signal);
    },
    renderCall: renderCall("list_knowledge_bases"),
  });

  pi.registerTool({
    name: "add_knowledge_item",
    label: "Add Knowledge Item",
    description: "Add durable, reusable knowledge to a selected knowledge base. Only use when the user explicitly asks to save information or clearly approves capture.",
    parameters: Type.Object({
      kb_id: Type.String(),
      title: Type.String(),
      content: Type.String(),
      scope: Type.Optional(Scope),
      tags: Type.Optional(Type.Array(Type.String())),
      metadata: Type.Optional(Type.Record(Type.String(), Type.Unknown())),
    }),
    async execute(_id, params, signal, _update, ctx) {
      const args = ["add", "--kb", params.kb_id, "--title", params.title, "--content", params.content];
      appendRepeated(args, "--tag", params.tags);
      if (params.metadata) args.push("--metadata", JSON.stringify(params.metadata));
      return run(withScope(params.scope, args), ctx.cwd, signal);
    },
    renderCall(args, theme) {
      return renderCall("add_knowledge_item", args.title)(args, theme);
    },
  });

  pi.registerTool({
    name: "delete_knowledge_item",
    label: "Delete Knowledge Item",
    description: "Permanently delete one knowledge item. Use only when the user explicitly requests deletion.",
    parameters: Type.Object({ kb_id: Type.String(), item_id: Type.String(), scope: Type.Optional(Scope) }),
    async execute(_id, params, signal, _update, ctx) {
      return run(withScope(params.scope, ["delete", "--kb", params.kb_id, "--id", params.item_id]), ctx.cwd, signal);
    },
    renderCall(args, theme) {
      return renderCall("delete_knowledge_item", `${args.kb_id}:${args.item_id}`)(args, theme);
    },
  });

  pi.registerTool({
    name: "index_knowledge",
    label: "Index Knowledge",
    description: "Backfill or rebuild semantic indexes for one knowledge base or all knowledge bases.",
    parameters: Type.Object({
      kb_id: Type.Optional(Type.String()),
      scope: Type.Optional(Scope),
      rebuild: Type.Optional(Type.Boolean()),
      all: Type.Optional(Type.Boolean()),
    }),
    async execute(_id, params, signal, _update, ctx) {
      const args = ["index", "--quiet"];
      if (params.all) args.push("--all");
      else if (params.kb_id) args.push("--kb", params.kb_id);
      if (params.rebuild) args.push("--rebuild");
      return run(withScope(params.scope, args), ctx.cwd, signal);
    },
    renderCall: renderCall("index_knowledge"),
  });

  pi.registerCommand("knowledger-status", {
    description: "Check the Knowledger binary and active storage context",
    handler: async (_args, ctx) => {
      try {
        const version = await pi.exec(binary, [...baseArgs(), "version"], { cwd: ctx.cwd, timeout: 10_000 });
        if (version.code !== 0) throw new Error(version.stderr.trim() || `exit code ${version.code}`);
        const config = configPath ? `, config ${configPath}` : "";
        ctx.ui.notify(`${version.stdout.trim() || binary} (${binary}${config})`, "info");
      } catch (error) {
        ctx.ui.notify(`Knowledger unavailable: ${error instanceof Error ? error.message : String(error)}`, "error");
      }
    },
  });
}
