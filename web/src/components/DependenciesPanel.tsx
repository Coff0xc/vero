import React, { useState, useEffect } from 'react';

interface Dependency {
  binary: string;
  display_name: string;
  installed: boolean;
  version?: string;
  install_hint?: string;
}

interface DependenciesResponse {
  dependencies: Dependency[];
  missing_count: number;
  all_ready: boolean;
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

  return (
    <div className="p-6 space-y-6">
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
          <li>• 工具缺失时 Agent 会自动跳过该工具, 不会中断战役</li>
        </ul>
      </div>
    </div>
  );
}
