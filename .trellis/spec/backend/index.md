# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This directory contains guidelines for backend development. Fill in each file with your project's specific conventions.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | To fill |
| [Database Guidelines](./database-guidelines.md) | ORM patterns, queries, migrations | Partial — serverColumns/scanServer alignment, migration helpers, config source-of-truth |
| [Save-File Handling](./save-file-handling.md) | Palworld save parsing, location, caching, DTO mapping, /save endpoints | Filled — palsave lib + internal/api/save_* |
| [Mod-Handling](./mod-handling.md) | Workshop download (login required), copy-deploy, PalModSettings.ini, UpdateMods pipeline, concurrency | Filled — internal/palmod + steamcmd/workshop + process.UpdateMods |
| [Platform & Deployment](./platform-and-deployment.md) | Cross-platform (Win/Linux) build-tag layout, Steam client symlinks, Docker run mode, .dockerignore hygiene, OS resource sampling (gopsutil sysstat) | Filled — internal/process, internal/steamcmd, internal/sysstat, Dockerfile |
| [Error Handling](./error-handling.md) | Error types, handling strategies | To fill |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | To fill |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging, log levels | To fill |

---

## How to Fill These Guidelines

For each guideline file:

1. Document your project's **actual conventions** (not ideals)
2. Include **code examples** from your codebase
3. List **forbidden patterns** and why
4. Add **common mistakes** your team has made

The goal is to help AI assistants and new team members understand how YOUR project works.

---

**Language**: All documentation should be written in **English**.
