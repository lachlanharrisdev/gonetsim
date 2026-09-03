# GoNetSim AI Usage Policy

Artificial Intelligence (LLMs specifically) can serve as a productivity tool for
developers, but it is not a substitute for human reasoning. GoNetSim enforces a 
strict boundary on where AI is permitted in this codebase to ensure code 
quality, security, and maintainability.

AI is permitted for formatting, adapting, and testing code, but it must never 
author the core logic.

## Permitted AI Usage

You may use AI tools to accelerate the structural and supplementary parts of 
development:

* WRITING TESTS Generating unit tests, table-driven test boilerplate, and mock 
  data to validate human-written logic.

* REFACTORING Cleaning up, formatting, linting, or optimising existing code 
  without changing its underlying functional behaviour.

* ADAPTING CODE Porting or adapting openly licensed third-party code to fit 
  GoNetSim's architecture, provided you verify licence compatibility and 
  maintain proper attribution.

* DOCUMENTATION Generating docstrings, comments, or documentation scaffolding 
  based on your existing code AND in the style of existing comments

## Prohibited AI Usage

Do not submit Pull Requests where AI was used for the following:

* CORE LOGIC AI must not architect, design, or implement the actual network 
  simulation logic, protocol handling, state management, or feature foundations. 

* UNVERIFIED COMMITS Do not submit AI-generated code without comprehensively 
  understanding its operation line-by-line. 

* AUTOMATED PRs Mass-generated or completely automated PRs driven by AI agents 
  will be closed immediately.

## Pull Request Requirements

If you utilise AI to generate tests, refactor components, or adapt third-party code, you must explicitly state this in your Pull Request description.

GoNetSim is a networking simulator where precision, security, and low-level understanding are critical. Code logic must remain human-authored, human-reviewed, and human-owned. Contributions that lack coherent architectural reasoning or demonstrate an over-reliance on generative AI for core logic will not be merged.