package main

import (
	"fmt"

	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/tools"
)

// ---------- P1: Metasploit RPC 命令行入口 ----------

func runMSFSearch(query string) {
	fmt.Printf("Metasploit Exploit Search: %s\n\n", query)

	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)

	tool, ok := reg.Get("msf_search")
	if !ok {
		fmt.Println("msf_search 未注册 (需要 ExploitPack)")
		return
	}

	args := map[string]any{
		"query":    query,
		"msf_url":  "http://127.0.0.1:55553",
		"username": "msf",
		"password": "password",
	}

	fmt.Println("连接到 msfrpcd (127.0.0.1:55553)...")
	res := tool.Run(args)

	if !res.Success {
		fmt.Printf("失败: %s\n", res.Stderr)
		return
	}

	fmt.Println("结果:\n" + res.Stdout)

	// 解析观测
	if tool.Parse != nil {
		obs := tool.Parse(res.Stdout, args)
		fmt.Printf("\n提取到 %d 个 exploit 模块:\n", len(obs))
		for _, o := range obs {
			fmt.Printf("  - %s\n", o.Label)
		}
	}
}

// ---------- P2: Cloud 命令行入口 ----------

func runCloudAWS() {
	fmt.Println("AWS IMDS Metadata Extraction")

	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)

	tool, ok := reg.Get("aws_imds_enum")
	if !ok {
		fmt.Println("aws_imds_enum 未注册 (需要 CloudPack)")
		return
	}

	fmt.Println("访问 http://169.254.169.254/latest/meta-data/...")
	res := tool.Run(map[string]any{})

	if !res.Success {
		fmt.Printf("失败: %s\n(需在 AWS EC2 实例内运行)\n", res.Stderr)
		return
	}

	fmt.Println("结果:\n" + res.Stdout)

	if tool.Parse != nil {
		obs := tool.Parse(res.Stdout, map[string]any{})
		fmt.Printf("\n提取到 %d 个观测:\n", len(obs))
		for _, o := range obs {
			fmt.Printf("  [%s] %s\n", o.Kind, o.Label)
		}
	}
}

func runCloudAzure() {
	fmt.Println("Azure IMDS Metadata Extraction")

	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)

	tool, ok := reg.Get("azure_imds_enum")
	if !ok {
		fmt.Println("azure_imds_enum 未注册 (需要 CloudPack)")
		return
	}

	fmt.Println("访问 Azure IMDS API...")
	res := tool.Run(map[string]any{})

	if !res.Success {
		fmt.Printf("失败: %s\n(需在 Azure VM 内运行)\n", res.Stderr)
		return
	}

	fmt.Println("结果:\n" + res.Stdout)

	if tool.Parse != nil {
		obs := tool.Parse(res.Stdout, map[string]any{})
		fmt.Printf("\n提取到 %d 个观测:\n", len(obs))
		for _, o := range obs {
			fmt.Printf("  [%s] %s\n", o.Kind, o.Label)
		}
	}
}

func runCloudGCP() {
	fmt.Println("GCP IMDS Metadata Extraction")

	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)

	tool, ok := reg.Get("gcp_imds_enum")
	if !ok {
		fmt.Println("gcp_imds_enum 未注册 (需要 CloudPack)")
		return
	}

	fmt.Println("访问 GCP Metadata Server...")
	res := tool.Run(map[string]any{})

	if !res.Success {
		fmt.Printf("失败: %s\n(需在 GCP Compute Engine 实例内运行)\n", res.Stderr)
		return
	}

	fmt.Println("结果:\n" + res.Stdout)

	if tool.Parse != nil {
		obs := tool.Parse(res.Stdout, map[string]any{})
		fmt.Printf("\n提取到 %d 个观测:\n", len(obs))
		for _, o := range obs {
			fmt.Printf("  [%s] %s\n", o.Kind, o.Label)
		}
	}
}

func runCloudS3(bucket string) {
	fmt.Printf("S3 Bucket Public Access Check: %s\n\n", bucket)

	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)

	tool, ok := reg.Get("s3_bucket_enum")
	if !ok {
		fmt.Println("s3_bucket_enum 未注册 (需要 CloudPack)")
		return
	}

	args := map[string]any{"bucket": bucket}
	res := tool.Run(args)

	if !res.Success {
		fmt.Printf("失败: %s\n", res.Stderr)
		return
	}

	fmt.Println("结果:\n" + res.Stdout)

	if tool.Parse != nil {
		obs := tool.Parse(res.Stdout, args)
		if len(obs) > 0 {
			fmt.Printf("\n发现:\n")
			for _, o := range obs {
				fmt.Printf("  [%s] %s\n", o.Kind, o.Label)
			}
		} else {
			fmt.Println("\n未发现公开访问")
		}
	}
}

// ---------- P3: Container 命令行入口 ----------

func runContainerEscape() {
	fmt.Println("Docker Container Escape Detection")

	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)

	tool, ok := reg.Get("docker_escape_check")
	if !ok {
		fmt.Println("docker_escape_check 未注册 (需要 ContainerPack)")
		return
	}

	fmt.Println("检测容器逃逸向量...")
	res := tool.Run(map[string]any{})

	if !res.Success {
		fmt.Printf("失败: %s\n(需在 Docker 容器内运行)\n", res.Stderr)
		return
	}

	fmt.Println("结果:\n" + res.Stdout)

	if tool.Parse != nil {
		obs := tool.Parse(res.Stdout, map[string]any{})
		if len(obs) > 0 {
			fmt.Printf("\n发现 %d 个逃逸向量:\n", len(obs))
			for _, o := range obs {
				fmt.Printf("  %s\n", o.Label)
			}
		} else {
			fmt.Println("\n未发现明显逃逸向量 (安全容器配置)")
		}
	}
}

func runK8sSA() {
	fmt.Println("Kubernetes ServiceAccount Extraction")

	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)

	tool, ok := reg.Get("k8s_sa_enum")
	if !ok {
		fmt.Println("k8s_sa_enum 未注册 (需要 ContainerPack)")
		return
	}

	fmt.Println("提取 ServiceAccount token...")
	res := tool.Run(map[string]any{"api_url": "https://kubernetes.default.svc"})

	if !res.Success {
		fmt.Printf("失败: %s\n(需在 Kubernetes pod 内运行)\n", res.Stderr)
		return
	}

	fmt.Println("结果:\n" + res.Stdout)

	if tool.Parse != nil {
		obs := tool.Parse(res.Stdout, map[string]any{})
		if len(obs) > 0 {
			fmt.Printf("\n提取到 %d 个观测:\n", len(obs))
			for _, o := range obs {
				fmt.Printf("  [%s] %s\n", o.Kind, o.Label)
			}
		}
	}
}
