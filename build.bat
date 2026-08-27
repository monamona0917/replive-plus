@echo off
setlocal EnableExtensions

cd /d "%~dp0"
set "ROOT=%CD%"
set "WEB_DIR=%ROOT%\replive-web-pro"
set "OUTPUT_DIR=%ROOT%\dist"
set "GOPROXY=https://goproxy.cn,direct"
set "GOSUMDB=sum.golang.google.cn"

if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

where go >nul 2>&1
if errorlevel 1 goto missing_go

where node >nul 2>&1
if errorlevel 1 goto missing_node

where npm >nul 2>&1
if errorlevel 1 goto missing_node

if not exist "%WEB_DIR%\node_modules" (
    echo Installing frontend dependencies...
    pushd "%WEB_DIR%"
    call npm ci --no-audit --no-fund
    if errorlevel 1 (
        popd
        goto failed
    )
    popd
)

if not exist "%ROOT%\vendor\modules.txt" (
    echo Go dependencies are missing. Downloading...
    go mod vendor
    if errorlevel 1 goto failed
)

echo Building frontend...
pushd "%WEB_DIR%"
call npm run build
if errorlevel 1 (
    popd
    goto failed
)
popd

echo Building backend...
go build -mod=vendor -trimpath -ldflags="-s -w" -o "%OUTPUT_DIR%\replive-plus.exe" .
if errorlevel 1 goto failed

echo Building frontend executable...
go build -mod=vendor -trimpath -ldflags="-s -w" -o "%OUTPUT_DIR%\replive-plus-web.exe" .\replive-web-pro
if errorlevel 1 goto failed

echo.
echo Build completed successfully.
echo Output directory: %OUTPUT_DIR%
echo   replive-plus.exe
echo   replive-plus-web.exe
echo.
pause
exit /b 0

:missing_go
echo Go was not found. Install Go 1.24 or newer, then run this file again.
echo Download: https://go.dev/dl/
pause
exit /b 1

:missing_node
echo Node.js and npm were not found. Install Node.js 20 or newer, then run this file again.
echo Download: https://nodejs.org/
pause
exit /b 1

:failed
echo.
echo Build failed. Read the error above and fix it before running this file again.
pause
exit /b 1
