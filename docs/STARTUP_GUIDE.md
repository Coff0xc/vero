# 启动服务指南

## 快速启动

### Windows
双击运行 `start_services.bat`，或在命令行执行：
```cmd
start_services.bat
```

### Linux/Mac
```bash
bash start_services.sh
```

---

## 手动启动

### 方式 1: 两个终端窗口

**终端 1 - 后端**:
```bash
cd D:\A\github-project-public\redteam-agent
./vero.exe -port 8080
```

**终端 2 - 前端**:
```bash
cd D:\A\github-project-public\redteam-agent/web
npm run dev
```

### 方式 2: 后台运行

**Windows (PowerShell)**:
```powershell
# 启动后端
Start-Process -FilePath ".\vero.exe" -ArgumentList "-port 8080" -WindowStyle Hidden

# 启动前端
cd web
Start-Process -FilePath "npm" -ArgumentList "run dev" -WindowStyle Hidden
```

**Linux/Mac**:
```bash
# 启动后端
./vero.exe -port 8080 > /tmp/vero-backend.log 2>&1 &

# 启动前端
cd web && npm run dev > /tmp/vero-frontend.log 2>&1 &
```

---

## 访问地址

启动成功后访问：

- **前端 UI**: http://localhost:5173 (或 5174 如果端口被占用)
- **后端 API**: http://localhost:8080
- **依赖检测**: http://localhost:8080/api/dependencies
- **配置 API**: http://localhost:8080/api/config

---

## 检查服务状态

### Windows
```cmd
netstat -ano | findstr "8080 5173"
```

### Linux/Mac
```bash
netstat -tuln | grep -E "8080|5173"
# 或
lsof -i :8080
lsof -i :5173
```

---

## 停止服务

### 查找进程
```bash
# Windows
tasklist | findstr "vero node"

# Linux/Mac
ps aux | grep -E "vero|vite"
```

### 终止进程
```bash
# Windows
taskkill /F /IM vero.exe
taskkill /F /IM node.exe

# Linux/Mac
killall vero.exe
killall node
```

---

## 常见问题

### 1. 端口被占用
**现象**: `bind: address already in use`

**解决**:
```bash
# 查找占用进程
netstat -ano | findstr "8080"  # Windows
lsof -i :8080                  # Linux/Mac

# 终止进程
taskkill /F /PID <PID>         # Windows
kill -9 <PID>                  # Linux/Mac
```

### 2. 前端无法连接后端
**检查**:
- 后端是否启动: `curl http://localhost:8080/api/config`
- CORS 配置是否正确
- 防火墙是否拦截

### 3. npm 依赖缺失
**解决**:
```bash
cd web
npm install
npm run dev
```

---

## 开发模式 vs 生产模式

### 开发模式 (当前)
- 前端: Vite 开发服务器 (热更新)
- 后端: 直接运行 `vero.exe`
- 适合开发调试

### 生产模式
```bash
# 1. 构建前端
cd web && npm run build

# 2. 启动后端 (自动服务前端静态文件)
cd ..
./vero.exe -port 8080
```

访问: http://localhost:8080 (后端同时提供前端)

---

## 日志查看

### 实时日志
```bash
# 后端日志 (如果后台运行)
tail -f /tmp/vero-backend.log

# 前端日志
tail -f /tmp/vero-frontend.log
```

### 浏览器控制台
按 `F12` 打开开发者工具查看前端日志

---

## 快速验证

服务启动后，执行：

```bash
# 测试后端
curl http://localhost:8080/api/dependencies | jq .

# 测试前端
curl http://localhost:5173

# 完整测试
open http://localhost:5173  # Mac
start http://localhost:5173  # Windows
```

---

## 推荐工作流

1. **启动服务**:
   ```bash
   bash start_services.sh  # 或 start_services.bat
   ```

2. **打开浏览器**: http://localhost:5173

3. **开始使用**:
   - 点击「设置」→「工具依赖」查看工具状态
   - 在对话框输入目标启动战役
   - 实时查看攻击图和发现

4. **停止服务**: Ctrl+C 停止终端，或 `killall vero.exe node`

---

**提示**: 首次启动可能需要 5-10 秒初始化，请耐心等待。
