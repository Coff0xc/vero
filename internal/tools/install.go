// 工具自动下载安装器 —— 解决"工具列表齐全但本机缺二进制, 能力悬空"的问题。
//
// 设计原则:
//   - 只自动安装纯编译产物(无系统库依赖)的二进制: nuclei / ffuf。
//   - 版本与校验和白名单锁定(Dockerfile 同源 v3.3.9 / v2.1.0), 防供应链投毒。
//   - Python 系工具(nxc/impacket/pypykatz)走 pip --user 安装, 不污染系统环境。
//   - 安装到 <工作目录>/tools/bin, 进程内注入 PATH(仅本进程可见, 不动系统)。
package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	nucleiVersion = "v3.3.9"
	ffufVersion   = "v2.1.0"
)

// toolArtifact —— 一个可自动下载的二进制产物。
type toolArtifact struct {
	Binary  string // 安装后的可执行文件名(nuclei / ffuf)
	Kind    string // zip | targz
	URL     string
	Sha256  string // 该平台产物的 SHA256(硬编码白名单)
	InnerRe string // 解压后要重命名的文件匹配模式
}

// artifactFor —— 当前平台可安装的产物; 不支持的平台返回 nil(绝不静默装错架构)。
func artifactFor(name string) *toolArtifact {
	goos, arch := runtime.GOOS, runtime.GOARCH
	if arch != "amd64" {
		return nil // 只对 amd64 校验过, 其他架构拒绝自动安装
	}
	switch name {
	case "nuclei":
		switch goos {
		case "linux":
			return &toolArtifact{Binary: "nuclei", Kind: "zip",
				URL:    "https://github.com/projectdiscovery/nuclei/releases/download/" + nucleiVersion + "/nuclei_" + strings.TrimPrefix(nucleiVersion, "v") + "_linux_amd64.zip",
				Sha256: "dfecedc31364d70b7291b347c74fd4d1d3185d30301c025b7490717d29daf28a", InnerRe: "nuclei"}
		case "windows":
			return &toolArtifact{Binary: "nuclei.exe", Kind: "zip",
				URL:    "https://github.com/projectdiscovery/nuclei/releases/download/" + nucleiVersion + "/nuclei_" + strings.TrimPrefix(nucleiVersion, "v") + "_windows_amd64.zip",
				Sha256: "4ef66d52d747627ab597c8c327dcc9dacc537aa3a3293095ee02e6818d63a5ce", InnerRe: "nuclei"}
		}
	case "ffuf":
		switch goos {
		case "linux":
			return &toolArtifact{Binary: "ffuf", Kind: "targz",
				URL:    "https://github.com/ffuf/ffuf/releases/download/" + ffufVersion + "/ffuf_" + strings.TrimPrefix(ffufVersion, "v") + "_linux_amd64.tar.gz",
				Sha256: "fc2c82736c14dcbea4daf3d3cf3878c1c4773008ba45c2bc0fceba7d17b40bb5", InnerRe: "ffuf"}
		case "windows":
			return &toolArtifact{Binary: "ffuf.exe", Kind: "zip",
				URL:    "https://github.com/ffuf/ffuf/releases/download/" + ffufVersion + "/ffuf_" + strings.TrimPrefix(ffufVersion, "v") + "_windows_amd64.zip",
				Sha256: "c0aec0289f1963cfc38a204f9ebe97712441c670fa7d7aad145cf87b15f038d4", InnerRe: "ffuf"}
		}
	}
	return nil
}

// pipPackage —— Python 系工具: 通过 pip --user 安装(不污染系统环境)。
// 键为"工具名"(非二进制名); PipPackage 由此派生, 修旧 bug: 曾用
// ToolBinary(二进制名) 索引, 使 secretsdump/lsass_dump/sam_dump(ToolBinary=python3)
// 的 pip 提示从未生效。
var pipPackage = map[string]string{
	// D20 修复: NetExec(原 CrackMapExec) 的 PyPI 包名是 netexec(官方安装: pipx install netexec),
	// 二进制名才是 nxc —— 包名与二进制名是两回事, 用 nxc 当包名 pip 装必失败。
	"nxc":         "netexec",
	"impacket":    "impacket",
	"pypykatz":    "pypykatz",
	"secretsdump": "impacket",
	"lsass_dump":  "pypykatz",
	"sam_dump":    "impacket",
}

// BinDir —— 本地工具安装目录(<工作目录>/tools/bin, 绝对路径)。
// 必须绝对: Windows 上 LookPath 拒绝"相对路径条目"里的可执行文件。
func BinDir() string {
	dir := filepath.Join("tools", "bin")
	abs, err := filepath.Abs(dir)
	if err == nil {
		dir = abs
	}
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// EnsurePath —— 把 bin 目录注入进程 PATH(仅本进程, 不动系统 PATH)。
func EnsurePath() {
	bin := BinDir()
	path := os.Getenv("PATH")
	if !strings.Contains(path, bin) {
		_ = os.Setenv("PATH", bin+string(os.PathListSeparator)+path)
	}
}

// ToolBinary —— 工具名 -> 其依赖的可执行文件(公共映射, tooltest 验证也用它)。
func ToolBinary(toolName string) string {
	switch {
	case strings.HasPrefix(toolName, "nxc_"), toolName == "smb_enum", toolName == "kerberoast":
		return "nxc"
	case toolName == "http_probe", toolName == "exploit_sqli", toolName == "s3_bucket_enum",
		strings.HasSuffix(toolName, "imds_enum"):
		return "curl"
	case toolName == "web_vuln_scan":
		return "nuclei"
	case strings.HasPrefix(toolName, "ffuf_"):
		return "ffuf"
	case toolName == "nmap_scan":
		return "nmap"
	case toolName == "docker_escape_check", toolName == "k8s_node_exploit":
		return "docker"
	case toolName == "secretsdump", toolName == "sam_dump":
		return "secretsdump.py" // D27: 与 deps.go 检测一致(impacket 脚本), 而非笼统的 python3
	case toolName == "lsass_dump":
		return "pypykatz" // D27: 与 deps.go 检测一致
	case toolName == "port_scan":
		return "" // Go 原生实现, 无外部依赖
	}
	return ""
}

// InstallableBinary —— 某工具缺失的依赖二进制是否可自动下载。
func InstallableBinary(toolName string) string {
	switch ToolBinary(toolName) {
	case "nuclei", "ffuf":
		return ToolBinary(toolName)
	}
	return ""
}

// PipPackage —— 工具名 -> 其 pip 包名(安装与提示的单一事实来源)。
// 键为工具名(非二进制名); nxc 系(nxc_* 前缀 / smb_enum / kerberoast)统一由 netexec 包提供。
func PipPackage(toolName string) string {
	if pkg, ok := pipPackage[toolName]; ok {
		return pkg
	}
	if strings.HasPrefix(toolName, "nxc_") || toolName == "smb_enum" || toolName == "kerberoast" {
		return pipPackage["nxc"]
	}
	return ""
}

// InstallPipHint —— Python 依赖的安装提示命令(不自动 pip, 避免污染; 给出可复制命令)。
// 由 PipPackage 派生, 修 secretsdump/lsass_dump/sam_dump 无提示的 bug。
func InstallPipHint(toolName string) string {
	if pkg := PipPackage(toolName); pkg != "" {
		return "pip install --user " + pkg
	}
	return ""
}

// InstallType —— 工具的自动安装途径三态: binary | pip | none。
// binary: 可自动下载的独立二进制(nuclei/ffuf); pip: 走 pip --user 安装; none: 无自动途径。
func InstallType(toolName string) string {
	if InstallableBinary(toolName) != "" {
		return "binary"
	}
	if PipPackage(toolName) != "" {
		return "pip"
	}
	return "none"
}

// InstallBinary —— 下载 + 校验 + 解压 + 重命名到 bin 目录。返回安装路径。
func InstallBinary(name string) (string, error) {
	art := artifactFor(name)
	if art == nil {
		return "", fmt.Errorf("%s 当前平台 (%s/%s) 不支持自动安装, 请手动安装", name, runtime.GOOS, runtime.GOARCH)
	}
	if err := ensureChecksum(art); err != nil {
		return "", err
	}
	bin := BinDir()
	target := filepath.Join(bin, art.Binary)
	if fi, err := os.Stat(target); err == nil && fi.Size() > 0 {
		return target, nil // 已安装
	}

	tmp, err := os.CreateTemp("", "vero-dl-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	client := httpClient()
	fmt.Fprintf(os.Stderr, "[vero] 下载 %s (%s)…\n", name, art.URL)

	resp, err := client.Get(art.URL)
	if err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = tmp.Close()
		return "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), resp.Body); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("下载中断: %w", err)
	}
	_ = tmp.Close()

	sum := hex.EncodeToString(hasher.Sum(nil))
	if sum != art.Sha256 {
		return "", fmt.Errorf("校验失败: 期望 %s 实际 %s (拒绝安装, 防投毒)", art.Sha256, sum)
	}

	if err := extract(tmpName, art, target); err != nil {
		return "", err
	}
	if err := os.Chmod(target, 0o755); err != nil {
		return "", err
	}
	// D15: 安装后验证产物真实存在且非空(防目录条目匹配导致空文件谎报成功)。
	if fi, err := os.Stat(target); err != nil || fi.Size() == 0 {
		_ = os.Remove(target)
		return "", fmt.Errorf("安装校验失败: 产物 %s 不存在或为空 (D15)", target)
	}
	EnsurePath()
	return target, nil
}

// ensureChecksum —— 产物必须有白名单校验和; 未内建的平台/版本直接拒绝(不绕过)。
func ensureChecksum(art *toolArtifact) error {
	if art.Sha256 != "" {
		return nil
	}
	return fmt.Errorf("%s 该校验和未内建, 拒绝自动安装(可手动下载 %s 并放到 %s)", art.Binary, art.URL, BinDir())
}

// extract —— 按包类型解压, 把匹配 InnerRe 的文件改名为目标名。
func extract(archive string, art *toolArtifact, target string) error {
	inner := art.InnerRe
	switch art.Kind {
	case "zip":
		zr, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if !strings.Contains(f.Name, inner) {
				continue
			}
			// D14: 跳过目录条目(如 nuclei zip 里的顶层目录), 否则 Windows 上
			// 对目录句柄 Copy 报 "A device which does not exist" 且产物为空文件。
			if f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				_ = rc.Close()
				return err
			}
			_, err = io.Copy(out, rc)
			_ = rc.Close()
			_ = out.Close()
			return err
		}
		return fmt.Errorf("包内未找到 %s", inner)
	case "targz":
		f, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if !strings.Contains(hdr.Name, inner) {
				continue
			}
			// D14: 跳过目录条目, 防空产物谎报成功。
			if hdr.Typeflag == tar.TypeDir || hdr.FileInfo().IsDir() {
				continue
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			_ = out.Close()
			return err
		}
		return fmt.Errorf("包内未找到 %s", inner)
	}
	return fmt.Errorf("未知包类型 %s", art.Kind)
}

// CheckPip —— 某 Python 工具依赖是否已可用。
func CheckPip(binary string) bool {
	path, err := exec.LookPath(binary)
	return err == nil && path != ""
}

// pipTimeout —— 单次 pip 安装超时(10 分钟)。
const pipTimeout = 10 * time.Minute

// pipInterpreter —— 探测可用的 pip 解释器: 优先 python3, 其次 python, Windows 再试 py。
// 逐个验证 "<py> -m pip --version" 可用, 保证返回的解释器真正支持 -m pip。
func pipInterpreter() (string, error) {
	var lastErr error
	for _, c := range []string{"python3", "python", "py"} {
		if _, err := exec.LookPath(c); err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		out, err := exec.CommandContext(ctx, c, "-m", "pip", "--version").CombinedOutput()
		cancel()
		if err == nil && strings.Contains(string(out), "pip") {
			return c, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("未找到可用的 pip 解释器: %w", lastErr)
	}
	return "", fmt.Errorf("未找到 python3/python/py 解释器, 请先安装 Python")
}

// InstallPip —— 通过 pip --user 安装 Python 系工具, 返回解析到的主脚本绝对路径(解析不到为空串)。
// 解析链: python3 -m pip -> python -m pip -> py -m pip(Windows);
// 命令: <py> -m pip install --user --disable-pip-version-check <pkg>, 超时 10 分钟;
// 输出含 externally-managed-environment(PEP 668)时追加 --break-system-packages 重试一次;
// 成功后把用户 scripts 目录注入进程 PATH(仅本进程, 不动系统)。
func InstallPip(name string) (string, error) {
	pkg := PipPackage(name)
	if pkg == "" {
		return "", fmt.Errorf("%s 无可自动安装的 pip 包", name)
	}
	py, err := pipInterpreter()
	if err != nil {
		return "", err
	}

	base := []string{"-m", "pip", "install", "--user", "--disable-pip-version-check"}
	out, err := pipRun(py, append(append([]string{}, base...), pkg))
	if err != nil && strings.Contains(string(out), "externally-managed-environment") {
		// PEP 668: 系统托管 Python 拒绝裸 --user, 追加 --break-system-packages 重试一次。
		out, err = pipRun(py, append(append([]string{}, base...), "--break-system-packages", pkg))
	}
	if err != nil {
		return "", fmt.Errorf("pip 安装 %s 失败: %s", pkg, truncateOut(out))
	}

	EnsureUserPath() // 装完注入用户 scripts 目录, 新工具立即可用
	return pipScriptPath(name), nil
}

// pipRun —— 执行一次 pip 命令并捕获合并输出, 超时 10 分钟。
func pipRun(py string, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), pipTimeout)
	defer cancel()
	return exec.CommandContext(ctx, py, args...).CombinedOutput()
}

// truncateOut —— 截断命令输出(错误信息过长会撑爆响应体)。
func truncateOut(out []byte) string {
	s := strings.TrimSpace(string(out))
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// userScriptsDir —— pip --user 的 scripts 目录:
// 类 UNIX ~/.local/bin; Windows %APPDATA%\Python\Python3xx\Scripts。
func userScriptsDir() string {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return ""
		}
		matches, _ := filepath.Glob(filepath.Join(appData, "Python", "Python3*", "Scripts"))
		if len(matches) == 0 {
			return ""
		}
		sort.Strings(matches)
		return matches[len(matches)-1] // 取最新 Python3xx
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "bin")
}

// EnsureUserPath —— 把 pip --user 的 scripts 目录注入进程 PATH(仅本进程, 不动系统)。
func EnsureUserPath() {
	dir := userScriptsDir()
	if dir == "" {
		return
	}
	path := os.Getenv("PATH")
	if !strings.Contains(path, dir) {
		_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+path)
	}
}

// pipEntrypoint —— 某工具 pip 安装后的主脚本名(尽力而为, 用于路径解析)。
func pipEntrypoint(name string) string {
	switch {
	case name == "nxc", strings.HasPrefix(name, "nxc_"), name == "smb_enum", name == "kerberoast":
		return "nxc"
	case name == "pypykatz", name == "lsass_dump":
		return "pypykatz"
	case name == "impacket", name == "secretsdump", name == "sam_dump":
		return "secretsdump"
	}
	return ""
}

// pipScriptPath —— 解析 pip 安装后的主脚本绝对路径; 解析不到返回空串。
// 依次查: 用户 scripts 目录(直配入口脚本) -> 进程 PATH(LookPath, 依赖 EnsureUserPath 已注入)。
func pipScriptPath(name string) string {
	entry := pipEntrypoint(name)
	if entry == "" {
		return ""
	}
	candidates := []string{entry}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, entry+".exe", entry+".py")
	}
	for _, c := range candidates {
		if dir := userScriptsDir(); dir != "" {
			full := filepath.Join(dir, c)
			if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
				return full
			}
		}
		if p, err := exec.LookPath(c); err == nil && p != "" {
			return p
		}
	}
	return ""
}

// PipInstallCommand —— 返回 pip 安装命令的可读描述(如 "python3 -m pip install --user netexec")。
func PipInstallCommand(name string) string {
	pkg := PipPackage(name)
	if pkg == "" {
		return ""
	}
	py := "python3"
	if c, err := pipInterpreter(); err == nil {
		py = c
	}
	return fmt.Sprintf("%s -m pip install --user %s", py, pkg)
}

// httpClient —— 带代理识别的下载客户端: 环境变量优先, Windows 兜底读 IE 注册表代理
// (用户常见场景: Clash 等只设了注册表代理, Go 默认不认, 下载必然超时)。
func httpClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute, Transport: &http.Transport{Proxy: proxyFor}}
}

// proxyFor —— 解析代理: HTTPS_PROXY/HTTP_PROXY 环境变量 -> Windows IE 注册表代理。
func proxyFor(req *http.Request) (*url.URL, error) {
	if p, err := http.ProxyFromEnvironment(req); err == nil && p != nil {
		return p, nil
	}
	return windowsProxy()
}
