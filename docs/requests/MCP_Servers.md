Yes, there are many free and open-source Model Context Protocol (MCP) servers that connect LLMs to tools for design, UI, image generation, asset creation, and more. Most are freely available to install and use, though some may require API keys for external services or access to specific local software.

---

🎨 Design & UI Integration

These servers bridge LLMs with popular design tools.

Server Description Requirements Source
Cursor Talk to Figma MCP Read and modify Figma designs using natural language. No explicit API keys needed. Requires Figma plugin and Cursor (or other MCP client).
Framelink Figma MCP Server Fetches and simplifies Figma file data for AI coding tools. Figma API access token.
Figma MCP Server with Chunking Handles large Figma files efficiently with chunking/pagination. Figma API access token.
MCP Figma to React Converter Converts Figma designs directly into React components. Figma API token.
Illustrator MCP Server Allows AI assistants to interact with Adobe Illustrator via JavaScript. macOS + Adobe Illustrator.
MCP‑Miro Connects to Miro whiteboards for creating sticky notes, shapes, etc. Miro OAuth token.
Photoshop Python API MCP Server Provides programmatic control of Adobe Photoshop. No specific API keys for basic setup.

---

🖼️ Image Generation & Editing

Servers that generate or manipulate images.

Server Description Requirements Source
Image Generation MCP Server (Replicate Flux) Generates images using the Replicate Flux model. Replicate API token (free tier available).
FLUX Image Generator Uses Black Forest Lab’s FLUX model for text-to-image generation. Black Forest Lab API key.
Stable Diffusion MCP Server Connects to a local Stable Diffusion WebUI for private, GPU‑accelerated image generation. Local Stable Diffusion WebUI setup.
ImageSorcery MCP Provides local image‑processing tools (crop, resize, OCR, object detection). Python 3.10+; downloads models locally.

---

🛠️ Asset Creation & Vector Graphics

Tools for generating and editing vector assets.

Server Description Requirements Source
SVGMaker MCP AI‑powered SVG generation, editing, and image‑to‑SVG conversion. SVGMaker API key (free tier likely).

---

🧰 Multi‑Purpose Toolboxes

Servers that bundle many capabilities into one.

Server Description Requirements Source
MCP‑Toolbox A comprehensive collection of tools for Figma, audio, memory, web search, and image generation (via Flux). Various API keys (Figma, Tavily, DuckDuckGo, BFL) depending on which tools you use.

---

💡 How to Get Started

1. Choose an MCP client: Popular options include Claude Desktop, Cursor, Windsurf, or Cline.
2. Install the server: Most servers can be installed via npm, pip, or by cloning their GitHub repository.
3. Configure your client: Add the server configuration to your client’s config file (e.g., claude_desktop_config.json).
4. Set up required access: For servers that need API keys, obtain the relevant token (many services offer free tiers).

Note on “free”: The MCP servers themselves are open‑source and free to use. However, some rely on external APIs (e.g., Replicate, Black Forest Lab, Figma, SVGMaker) that may have usage limits or costs beyond a free tier. Local servers (like Stable Diffusion MCP or ImageSorcery) run entirely on your hardware and incur no extra charges.

---

🔗 Useful Resources

· Awesome MCP Servers: A curated list of MCP servers.
· 14 MCP Servers for UI/UX Engineers (Snyk article): Detailed overview of design‑focused servers.
· MCP Servers registry: Browse servers by category.

By mixing and matching these free MCP servers, you can equip your LLM with a powerful suite of design, UI, image, and asset‑generation capabilities. Start with the servers that match your immediate needs (e.g., Figma integration for design, Stable Diffusion for local image generation) and expand your toolkit as your workflows evolve.

Main resources online:

https://mcpservers.org/all

https://snyk.io/articles/14-mcp-servers-for-ui-ux-engineers/

https://mcpservers.org/remote-mcp-servers

Next:

Here are free, open-source MCP servers, LSP servers, and embedding models you can integrate with your LLMs and AI CLI coding agents (like Google Gemini CLI, Aider, or Claude Code).

🛠️ Free MCP Servers

Model Context Protocol (MCP) servers let your AI agents access tools and data. The following are free and open-source, though some may require an API key for external services.

Server Description License / Cost Source
Filesystem Secure file operations with configurable access controls. Open‑source (free) Reference servers
Git Tools to read, search, and manipulate Git repositories. Open‑source (free) Reference servers
Fetch Web‑content fetching and conversion for efficient LLM usage. Open‑source (free) Reference servers
Memory Knowledge‑graph‑based persistent memory system. Open‑source (free) Reference servers
Time Time and timezone conversion capabilities. Open‑source (free) Reference servers
Chroma Embeddings, vector search, document storage, and full‑text search. Open‑source (free) Awesome MCP Servers
Qdrant MCP Vector Server Vector‑search server for similarity retrieval. Open‑source (free) Awesome MCP Servers
AWS Bedrock KB Retrieval Query Amazon Bedrock Knowledge Bases using natural language. Requires AWS account (pay‑per‑use) Awesome MCP Servers

How to use: Add the server’s configuration to your CLI agent’s config file (e.g., claude_desktop_config.json). Many servers are available via npm or pip.

💻 Free LSP Servers for AI Coding Agents

Language Server Protocol (LSP) servers provide code intelligence (completion, diagnostics, etc.). These can be used by AI coding agents to understand code structure.

Server Description License / Cost Source
LSP‑AI Open‑source language server that serves as a backend for AI‑powered functionality; supports in‑editor chatting, code completions, and works with any LSP‑compatible editor. Open‑source (free) GitHub – SilasMarvin/lsp‑ai
clangd Official LSP server for C/C++. Open‑source (free) clangd.llvm.org
pylsp Official LSP server for Python (formerly python‑language‑server). Open‑source (free) GitHub – python‑lsp/python‑lsp‑server
typescript‑language‑server LSP server for TypeScript/JavaScript. Open‑source (free) GitHub – typescript‑language‑server
rust‑analyzer LSP server for Rust. Open‑source (free) GitHub – rust‑analyzer
sumneko‑lua‑language‑server LSP server for Lua. Open‑source (free) GitHub – sumneko/lua‑language‑server

How to use: Install the LSP server locally and configure your editor or AI agent to connect to it. LSP‑AI is particularly designed for AI integration and supports multiple LLM backends (llama.cpp, Ollama, OpenAI‑compatible APIs, etc.).

🔤 Free Embedding Models

Embedding models convert text into vectors for semantic search, retrieval‑augmented generation (RAG), and other LLM tasks. The following are open‑source and can be run locally or via hosted APIs (some with free tiers).

Model Description License / Cost Source
Qwen3‑Embedding‑0.6B Multilingual, instruction‑aware, flexible output dimensions (32‑1024). Apache 2.0 (free) Hugging Face
EmbeddingGemma‑300M Lightweight, multilingual, optimized for on‑device deployment. Apache 2.0 (free) Hugging Face
Jina Embeddings v4 Universal, multimodal, multilingual (dense & multi‑vector). CC‑BY‑NC‑4.0 (non‑commercial) Jina AI
BGE‑M3 Multi‑functionality (dense, multi‑vector, sparse), multilingual, long‑context (8192 tokens). MIT (free) Hugging Face
all‑mpnet‑base‑v2 Sentence‑transformer model, good for English sentences/paragraphs. MIT (free) Hugging Face
gte‑multilingual‑base General‑purpose multilingual embedding model. MIT (free) Hugging Face
Nomic Embed Text V2 Matryoshka‑style embeddings, adjustable dimensionality. Apache 2.0 (free) Hugging Face

How to use: Run locally with libraries like sentence‑transformers, transformers, or via Ollama. For vector storage, pair with local databases (Chroma, FAISS, Qdrant) or use an MCP server like Chroma or Qdrant MCP Vector Server.

📝 Integration Summary

1. MCP servers extend your CLI agent with tools (files, Git, web, vectors, etc.). Start with the reference servers (Filesystem, Git, Fetch) for basic operations.
2. LSP servers give your agent deep code understanding. LSP‑AI is built for AI integration, while traditional LSPs (clangd, pylsp) provide language‑specific intelligence.
3. Embedding models enable semantic search and RAG. Choose based on language, size, and performance needs; run them locally or via free‑tier APIs.

For a completely free stack, combine Google Gemini CLI (generous free tier) with Filesystem, Git, and Chroma MCP servers, use LSP‑AI for code intelligence, and run BGE‑M3 or all‑mpnet‑base‑v2 locally for embeddings.

If you have a specific use case (e.g., “I need to search my codebase” or “I want to generate images”), let me know and I can recommend more targeted servers.

Integrating Language Server Protocol (LSP) servers with your LLMs and AI CLI agents can significantly enhance their code intelligence, enabling features like deep semantic understanding, autocompletion, and precise refactoring. Below is a comprehensive overview of free, open-source LSP servers and related tools suitable for AI workflows.

🧠 AI‑Specific LSP Servers

These servers are built with AI integration as a primary goal, often acting as a bridge between traditional LSP features and LLM backends.

Server Description Key Features for AI License / Cost
LSP‑AI An open‑source language server that serves as a backend for AI‑powered functionality in editors. It abstracts LLM integration and provides in‑editor chat, custom actions, and code completions. – Supports llama.cpp, Ollama, OpenAI‑compatible, Anthropic‑compatible, Gemini‑compatible, and Mistral AI FIM‑compatible APIs. – Works with any editor that supports LSP (VS Code, NeoVim, Emacs, Helix, etc.). – Provides structured context for AI agents. MIT (free)
OpenCode An open‑source AI coding agent that automatically loads the appropriate LSPs for the LLM being used. It operates in terminal, IDE, or desktop environments. – “LSP enabled: Automatically loads the right LSPs for the LLM”. – Supports 75+ LLM providers, including local models. – Multi‑session and privacy‑focused (no code storage). Open‑source (free)

📚 Traditional LSP Servers (by Language)

These are standard, language‑specific LSP servers that provide deep semantic understanding (go‑to‑definition, find references, etc.). Most are free and open‑source.

Language Recommended Server(s) Notes
Python pyright (Microsoft), palantir (Python LSP Server) Both are widely used; pyright is faster, palantir is more extensible.
JavaScript/TypeScript typescript‑language‑server (official), deno lsp (Deno) The TypeScript server is the standard; Deno’s LSP also supports TS/JS.
C/C++ clangd (LLVM), ccls, cquery clangd is the most active and recommended.
Rust rust‑analyzer The de‑facto standard for Rust.
Go gopls (Go team) Official Go language server.
Java eclipse‑jdt‑ls (Eclipse), java‑language‑server (Red Hat) Eclipse JDT LS is the most full‑featured.
C# omnisharp‑roslyn, csharp‑ls OmniSharp is the traditional choice; csharp‑ls is a newer alternative.
PHP phpactor (PHPactor), intelephense (proprietary) PHPactor is open‑source; Intelephense has a free tier with limitations.
Ruby solargraph Standard Ruby LSP.
Elixir elixir‑ls Official Elixir LSP.
Haskell haskell‑language‑server (HLS) The main Haskell server.
Shell (Bash) bash‑language‑server Provides linting, formatting, and completions.
Dockerfile docker‑language‑server Supports Dockerfile syntax.
YAML yaml‑language‑server Provides schema validation, completion.
XML lemminx The standard XML LSP.
Terraform terraform‑ls (Hashicorp) Official Terraform LSP.

Sources: The above list is curated from the “Awesome LSP Servers” GitHub repository and the official Microsoft LSP Implementations page.

🔌 MCP Servers with LSP‑Like Capabilities

These Model Context Protocol (MCP) servers expose code‑analysis tools to AI agents, often in a lighter‑weight, more secure manner than a full LSP server.

Server Description Use Case for AI Agents License
LSP Tools MCP Server A lightweight Node.js MCP server that provides regex‑based text‑search and directory‑listing tools. It is designed for “surgical precision” in text‑based tasks. – Finding exact pattern matches in code (e.g., for auditing, refactoring prep). – Security‑focused: only accesses explicitly allowed directories. MIT (free)
Neovim LSP MCP Server (mentioned in search results) An MCP server that bridges AI coding assistants with Neovim’s LSP capabilities. Allows AI agents to leverage Neovim’s built‑in LSP client for code navigation, definition lookup, etc. Likely open‑source
Semantic Analysis MCP Servers (e.g., isaacphi/mcp‑language‑server, Tritlo/lsp‑mcp) Full bridges to the Language Server Protocol, providing deep semantic tools (find all references, get definition, rename symbol). When your AI agent needs true language‑aware code intelligence, not just text search. Varies (often MIT)

🛠️ How to Integrate These LSPs with Your AI CLI Agents

Integration generally follows two paths:

1. Direct LSP Integration: Some AI CLI agents (like OpenCode) have built‑in LSP support and automatically load relevant servers. For custom agents, you can use an LSP client library (e.g., lsp‑client in Node.js, python‑lsp‑client) to communicate with the LSP server over JSON‑RPC.
2. Via MCP (Model Context Protocol): Many modern AI CLI agents (e.g., Claude Code, Google Gemini CLI, Cursor) support MCP. To add an LSP‑like tool:
   · Install the MCP server (e.g., lsp‑tools‑mcp).
   · Add its configuration to your agent’s MCP settings file (e.g., claude_desktop_config.json).
   · The agent will then discover the server’s tools (e.g., find_regex_position) and use them in its workflow.

💡 Recommendations for Different Workflows

· For AI‑centric coding assistants: Start with LSP‑AI—it’s designed specifically to bring LLM‑powered features (chat, completions, refactoring) into any LSP‑compatible editor.
· For language‑specific deep code intelligence: Use the traditional LSP servers listed above. They provide the most accurate semantic understanding for each language.
· For lightweight, secure code analysis in AI agents: The LSP Tools MCP Server is ideal for regex‑based search tasks without the overhead of a full LSP.
· For an all‑in‑one AI coding agent with built‑in LSP support: OpenCode automatically manages LSPs for you, making it a good “batteries‑included” option.

Note on “free”: The servers listed here are open‑source and free to use. However, some (like LSP‑AI) require you to bring your own LLM backend (which may have associated costs). Traditional LSP servers are typically completely free and do not require any external API.

For a more exhaustive list of LSP servers, you can browse the Microsoft LSP Implementations page or the Awesome LSP Servers repository.

If you have a specific language or use case in mind (e.g., “I need an LSP for Go that works well with AI refactoring”), I can help narrow down the options further.

Main resources:

https://microsoft.github.io/language-server-protocol/implementors/servers/

https://github.com/Hexlet/awesome-lsp-servers
