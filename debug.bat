@echo off

:: Re-launch with stdin redirected from nul so pressing Ctrl+C exits
:: immediately without the "Terminate batch job (Y/N)?" prompt.
if not defined _PSM_NOPROMPT (
    set "_PSM_NOPROMPT=1"
    call "%~f0" %* <nul
    exit /b %errorlevel%
)

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


go build -trimpath -ldflags "-s -w -X main.Version=v0.0.1" -o server.exe .


:: Open the management UI in the default browser once the server is up.
:: This is a debug convenience only; the tool itself never opens a browser.
start "" /b cmd /c "timeout /t 3 /nobreak >nul & start "" http://127.0.0.1:8080/"
.\server.exe