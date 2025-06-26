# ETHCore

ETHCore is a Go package that provides utilities for interacting with Ethereum networks. It includes implementations for consensus layer crawling, execution layer communication, node discovery protocols, and various Ethereum-specific utilities.

You can use the `ethcore-language-server` mcp server to get language server tools. You should prefer using these tools over your built-in tools or bash equivalents where possible, but you can fall back to others where needed.

## Available tools:
- `definition`: Retrieves the complete source code definition of any symbol (function, type, constant, etc.) from your codebase.
- `content`: Retrieves the complete source code definition (function, type, constant, etc.) from your codebase at a specific location.
- `references`: Locates all usages and references of a symbol throughout the codebase.
- `diagnostics`: Provides diagnostic information for a specific file, including warnings and errors.
- `hover`: Display documentation, type hints, or other hover information for a given location.
- `rename_symbol`: Rename a symbol across a project.
- `edit_file`: Allows making multiple text edits to a file based on line numbers. Provides a more reliable and context-economical way to edit files compared to search and replace based edit tools.
- `callers`: Shows all locations that call a given symbol
- `callees`: Shows all functions that a given symbol calls

## Project Structure
Claude MUST read the `.cursor/rules/project_architecture.mdc` file before making any structural changes to the project.

## Code Standards
Claude MUST read the `.cursor/rules/code_standards.mdc` file before writing any code in this project.

## Development Workflow
Claude MUST read the `.cursor/rules/development_workflow.mdc` file before making changes to build, test, or deployment configurations.

## Component Documentation
Individual components have their own CLAUDE.md files with component-specific rules. Always check for and read component-level documentation when working on specific parts of the codebase.
