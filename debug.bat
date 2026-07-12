@echo off
setlocal enabledelayedexpansion

echo ========================================
echo PalWorld Server Manager - Debug Script
echo ========================================
echo.

:: Save the root directory
set ROOT_DIR=%cd%

echo [1/4] Cleaning frontend build directories...
echo.
cd ui
if errorlevel 1 (
    echo Error: Cannot change to ui directory
    exit /b 1
)

if exist "out" (
    echo Removing ui/out...
    rmdir /s /q "out"
)

if exist ".next" (
    echo Removing ui/.next...
    rmdir /s /q ".next"
)

echo.
echo [2/4] Building frontend...
echo.

call bun run build
if errorlevel 1 (
    echo.
    echo Error: Frontend build failed
    cd "%ROOT_DIR%"
    exit /b 1
)

echo.
echo [3/4] Returning to root directory...
cd "%ROOT_DIR%"

echo.
echo [4/4] Starting Go backend...
echo.
echo Press Ctrl+C to stop the server
echo ========================================
echo.

go run .