import React, { useState, useEffect } from 'react';

interface Dependency {
  binary: string;
  display_name: string;
  installed: boolean;
  version?: string;
  install_hint?: string;
  platform?: string; // 新增: 工具平台标识
}

interface DependenciesResponse {
  dependencies: Dependency[];
  missing_count: number;
  all_ready: boolean;
  platform?: string; // 新增: 当前运行平台
  platform_tools?: {
    windows: number;
    linux: number;
    darwin: number;
    all: number;
  };
}

export function DependenciesPanel() {
  const [deps, setDeps] = useState<DependenciesResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchDependencies();
  }, []);

  const fetchDependencies = async () => {
    try {
      setLoading(true);
      const res = await fetch('/api/dependencies');
      if (!res.ok) throw new Error('获取依赖失败');
      const data = await res.json();
      setDeps(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : '未知错误');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="p-6 text-center">
        <div className="animate-spin w-8 h-8 border-2 border-amber-500 border-t-transparent rounded-full mx-auto" />
        <p className="mt-4 text-sm text-gray-500">加载工具依赖...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <p className="text-red-600">⚠ {error}</p>
          <button
            onClick={fetchDependencies}
            className="mt-2 text-sm text-red-700 underline"
          >
            重试
          </button>
        </div>
      </div>
    );
  }

  if (!deps) return null;

  const installed = deps.dependencies.filter(d => d.installed);
  const missing = deps.dependencies.filter(d => !d.installed);

  // 平台图标
  const platformIcon = {
    windows: '🪟',
    linux: '🐧',
    darwin: '🍎',
  };

  const platformName = {
    windows: 'Windows',
    linux: 'Linux',
    darwin: 'macOS',
  };

  return (
    <div className="p-6 space-y-6">
      {/* 平台信息卡片 */}
      {deps.platform && (
        <div className="bg-gradient-to-r from-blue-50 to-indigo-50 border-2 border-blue-200 rounded-lg p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="text-3xl">{platformIcon[deps.platform as keyof typeof platformIcon] || '💻'}</span>
              <div>
                <h3 className="font-semibold text-lg text-blue-900">
                  当前平台: {platformName[deps.platform as keyof typeof platformName] || deps.platform}
                </h3>
                {deps.platform_tools && (
                  <p className="text-sm text-blue-700 mt-1">
                    平台工具: {deps.platform === 'windows' ? deps.platform_tools.windows :
                              deps.platform === 'linux' ? deps.platform_tools.linux :
                              deps.platform_tools.darwin} 个专用 + {deps.platform_tools.all} 个通用
                  </p>
                )}
              </div>
            </div>
            <div className="text-right">
              <span className="text-xs text-blue-600 bg-blue-100 px-2 py-1 rounded">
                自动过滤不兼容工具
              </span>
            </div>
          </div>
        </div>
      )}

      {/* 总览卡片 */}
      <div className={`rounded-lg border-2 p-4 transition-all duration-300 ${
        deps.all_ready
          ? 'bg-green-50 border-green-300 shadow-sm'
          : 'bg-amber-50 border-amber-400 shadow-md'
      }`}>
        <div className="flex items-center justify-between">
          <div>
            <h3 className="font-semibold text-lg">
              {deps.all_ready ? '✓ 工具链就绪' : '⚠ 缺失工具'}
            </h3>
            <p className="text-sm text-gray-600 mt-1">
              {installed.length}/{deps.dependencies.length} 个工具已安装
              {deps.missing_count > 0 && ` · ${deps.missing_count} 个缺失`}
            </p>
          </div>
          <button
            onClick={fetchDependencies}
            disabled={loading}
            className="px-3 py-1.5 text-sm bg-white border border-gray-300 rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {loading ? '加载中...' : '刷新'}
          </button>
        </div>
      </div>

      {/* 缺失工具 (优先显示) */}
      {missing.length > 0 && (
        <div>
          <h4 className="font-medium mb-3 flex items-center gap-2">
            <span className="w-2 h-2 bg-red-500 rounded-full" />
            缺失工具 ({missing.length})
          </h4>
          <div className="space-y-3">
            {missing.map(dep => (
              <div
                key={dep.binary}
                className="bg-white border border-red-200 rounded-lg p-4 hover:shadow-sm transition-shadow"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-semibold text-red-600">
                        {dep.binary}
                      </span>
                      <span className="text-xs text-gray-500">
                        {dep.display_name}
                      </span>
                      {dep.platform && dep.platform !== 'all' && (
                        <span className="text-xs bg-purple-100 text-purple-700 px-1.5 py-0.5 rounded">
                          {platformName[dep.platform as keyof typeof platformName] || dep.platform}
                        </span>
                      )}
                    </div>
                    {dep.install_hint && (
                      <div className="mt-2 bg-gray-50 border border-gray-200 rounded p-2">
                        <p className="text-xs text-gray-600 mb-1">安装命令:</p>
                        <code className="text-xs font-mono text-gray-800 block select-all">
                          {dep.install_hint}
                        </code>
                      </div>
                    )}
                  </div>
                  <span className="text-red-500 font-bold text-xl">✗</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 已安装工具 */}
      {installed.length > 0 && (
        <div>
          <h4 className="font-medium mb-3 flex items-center gap-2">
            <span className="w-2 h-2 bg-green-500 rounded-full" />
            已安装工具 ({installed.length})
          </h4>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {installed.map(dep => (
              <div
                key={dep.binary}
                className="bg-white border border-green-200 rounded-lg p-3 hover:shadow-sm transition-shadow"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-semibold text-green-600 truncate">
                        {dep.binary}
                      </span>
                      <span className="text-green-500 font-bold">✓</span>
                    </div>
                    <p className="text-xs text-gray-500 mt-1 truncate">
                      {dep.display_name}
                      {dep.platform && dep.platform !== 'all' && (
                        <span className="ml-2 text-purple-600">
                          · {platformIcon[dep.platform as keyof typeof platformIcon]}
                          {platformName[dep.platform as keyof typeof platformName]}
                        </span>
                      )}
                    </p>
                    {dep.version && (
                      <p className="text-xs text-gray-400 mt-1 font-mono truncate">
                        {dep.version}
                      </p>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 帮助说明 */}
      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
        <h4 className="font-medium text-blue-900 mb-2">💡 说明</h4>
        <ul className="text-sm text-blue-800 space-y-1">
          <li>• 核心工具 (nuclei/nmap) 必须安装才能执行扫描</li>
          <li>• 场景工具 (aws/kubectl) 仅在相应场景激活时需要</li>
          <li>• <strong>平台专用工具</strong>自动根据系统过滤 (Windows/Linux互不干扰)</li>
          <li>• 工具缺失时 Agent 会自动跳过该工具, 不会中断战役</li>
        </ul>
      </div>
    </div>
  );
}
