# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## High-Level Code Architecture

This codebase includes the following structure:

### Directory Organization:

1. **App Directory**:
   - Contains `main.go`, likely the application entry point. Test functions are invoked manually within `main()`.

2. **src Directory**:
   - **Conversion Subdirectory**:
     - Includes format conversion scripts for Go (`request_convert.go`, `response_convert.go`) and Python (`request_converter.py`, `response_converter.py`).
   - **Core Subdirectory**:
     - Core components, such as `config.go` (for configuration), `constants.go` (application constants), `model_manager.go` (model management).
   - **Models Subdirectory**:
     - Contains `claude.go` for interactions or definitions related to the "Claude" model.

3. **Tests Directory**:
   - Houses `main.go` for testing various API behaviors.

4. **Root Files**:
   - Includes `go.mod` and `go.sum` for dependency management.

This modular organization separates conversion, core functionality, and model-specific logic, ensuring clarity.

## Common Development Commands

### Testing
- Individual test functions like `testBasicChat()` and `testStreamingChat()` are commented in `main.go`. To run a test:
  - Uncomment the desired function and execute the program via:
    ```
    go run main.go
    ```

### Build and Linting
- Not configured. Recommended tools for setup:
  - Build: Add `Dockerfile` or `Makefile`.
  - Linting: Integrate `golangci-lint`.

### Suggestions
- To automate testing:
  - Set up `go test` with proper test function definitions.