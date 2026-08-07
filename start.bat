@echo off
echo ========================================
echo Vero 渗透测试平台 - 启动脚本
echo ========================================
echo.

:: 检查是否已在运行
netstat -ano | findstr "8080" >nul
if %errorlevel% equ 0 (
    echo [警告] 后端端口 8080 已被占用
    echo 是否要停止现有进程? (Y/N)
    set /p choice=
    if /i "%choice%"=="Y" (
        for /f "tokens=5" %%a in ('netstat -ano ^| findstr "8080"') do taskkill /F /PID %%a 2>nul
        timeout /t 2 /nobreak >nul
    )
)

echo [1/3] 启动后端服务...
start "Vero后端" cmd /k "cd /d %~dp0 && vero.exe -port 8080"
timeout /t 3 /nobreak >nul

echo [2/3] 启动前端服务...
start "Vero前端" cmd /k "cd /d %~dp0web && npm run dev"
timeout /t 5 /nobreak >nul

echo [3/3] 打开浏览器...
start http://localhost:5173

echo.
echo ========================================
echo 服务已启动！
echo ========================================
echo 后端: http://localhost:8080
echo 前端: http://localhost:5173
echo.
echo 按任意键关闭本窗口 (服务将继续运行)
pause >nul
