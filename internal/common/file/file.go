package file

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/yinstall/internal/runner"
)

// FindAndDistribute 查找文件并分发到远程
// 查找顺序：1. 远程目标目录 2. 远程用户家目录 3. 本地目录列表
// 如果在本地找到，上传到远程用户家目录
// 返回远程文件路径
func FindAndDistribute(
	executor runner.Executor,
	filename string,
	localDirs []string,
	remoteDir string,
) (string, error) {
	// 判断是设备路径还是文件名
	if strings.HasPrefix(filename, "/dev/") {
		// 设备路径，直接返回
		return filename, nil
	}

	// 获取基础文件名
	baseName := filepath.Base(filename)

	// 获取远程用户家目录
	remoteHomeDir := "/root" // 默认值
	result, _ := executor.Execute("echo $HOME", false)
	if result != nil && strings.TrimSpace(result.GetStdout()) != "" {
		remoteHomeDir = strings.TrimSpace(result.GetStdout())
	}

	// 1. 先在远程目标目录查找
	if remoteDir != "" {
		remotePath := filepath.Join(remoteDir, baseName)
		result, _ := executor.Execute(fmt.Sprintf("test -f '%s' && echo 'exists'", remotePath), false)
		if result != nil && strings.Contains(result.GetStdout(), "exists") {
			return remotePath, nil
		}
	}

	// 2. 在远程用户家目录查找
	remoteHomePath := filepath.Join(remoteHomeDir, baseName)
	result, _ = executor.Execute(fmt.Sprintf("test -f '%s' && echo 'exists'", remoteHomePath), false)
	if result != nil && strings.Contains(result.GetStdout(), "exists") {
		return remoteHomePath, nil
	}

	// 3. 在本地目录列表查找
	var localPath string

	// 优先在配置的本地目录中查找
	for _, dir := range localDirs {
		// 尝试基础文件名
		candidate := filepath.Join(dir, baseName)
		if _, err := os.Stat(candidate); err == nil {
			localPath = candidate
			break
		}

		// 如果 filename 包含子路径（相对路径），也尝试
		if filename != baseName && !filepath.IsAbs(filename) {
			candidate = filepath.Join(dir, filename)
			if _, err := os.Stat(candidate); err == nil {
				localPath = candidate
				break
			}
		}
	}

	// 如果 filename 是绝对路径且未在配置目录中找到，检查该路径是否存在（向后兼容）
	if localPath == "" && filepath.IsAbs(filename) {
		if _, err := os.Stat(filename); err == nil {
			localPath = filename
		}
	}

	if localPath == "" {
		return "", fmt.Errorf("file '%s' not found in remote dirs ['%s', '%s'] or local dirs %v", filename, remoteDir, remoteHomeDir, localDirs)
	}

	// 4. 上传文件到远程用户家目录
	uploadPath := filepath.Join(remoteHomeDir, baseName)

	if err := executor.Upload(localPath, uploadPath); err != nil {
		return "", fmt.Errorf("failed to upload '%s' to '%s': %w", localPath, uploadPath, err)
	}

	// 验证上传成功
	result, _ = executor.Execute(fmt.Sprintf("test -f '%s' && echo 'exists'", uploadPath), false)
	if result == nil || !strings.Contains(result.GetStdout(), "exists") {
		return "", fmt.Errorf("file upload verification failed for '%s'", uploadPath)
	}

	return uploadPath, nil
}

// FileExists 检查远程文件是否存在
func FileExists(executor runner.Executor, path string) bool {
	result, _ := executor.Execute(fmt.Sprintf("test -f '%s' && echo 'exists'", path), false)
	return result != nil && strings.Contains(result.GetStdout(), "exists")
}

// DirExists 检查远程目录是否存在
func DirExists(executor runner.Executor, path string) bool {
	result, _ := executor.Execute(fmt.Sprintf("test -d '%s' && echo 'exists'", path), false)
	return result != nil && strings.Contains(result.GetStdout(), "exists")
}

// EnsureDir 确保远程目录存在
func EnsureDir(executor runner.Executor, path string, sudo bool) error {
	result, err := executor.Execute(fmt.Sprintf("mkdir -p '%s'", path), sudo)
	if err != nil {
		return err
	}
	if result != nil && result.GetExitCode() != 0 {
		return fmt.Errorf("failed to create directory '%s': %s", path, result.GetStderr())
	}
	return nil
}

// IsDevicePath 判断是否为设备路径
func IsDevicePath(path string) bool {
	return strings.HasPrefix(path, "/dev/")
}

// IsISOFile 判断是否为 ISO 文件
func IsISOFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".iso")
}

// FindLatestDBPackage 自动查找最新版本的数据库软件包
// 软件包格式: yashandb-23.4.7.100-linux-x86_64.tar.gz 或 yashandb-23.4.7.100-linux-aarch64.tar.gz
// 返回找到的软件包路径（远程或本地）
func FindLatestDBPackage(
	executor runner.Executor,
	localDirs []string,
	remoteDir string,
) (string, error) {
	// 获取远程架构
	arch := "x86_64" // 默认
	result, _ := executor.Execute("uname -m", false)
	if result != nil && strings.TrimSpace(result.GetStdout()) != "" {
		remoteArch := strings.TrimSpace(result.GetStdout())
		if remoteArch == "aarch64" || remoteArch == "arm64" {
			arch = "aarch64"
		}
	}

	pattern := fmt.Sprintf(`yashandb-(\d+\.\d+\.\d+\.\d+)-linux-%s\.tar\.gz`, arch)
	re := regexp.MustCompile(pattern)

	// 1. 先在远程目录查找
	var remotePackages []string
	if remoteDir != "" {
		result, _ := executor.Execute(fmt.Sprintf("ls -1 %s/yashandb-*-linux-%s.tar.gz 2>/dev/null || true", remoteDir, arch), false)
		if result != nil && result.GetStdout() != "" {
			files := strings.Split(strings.TrimSpace(result.GetStdout()), "\n")
			for _, f := range files {
				f = strings.TrimSpace(f)
				if f != "" && re.MatchString(filepath.Base(f)) {
					remotePackages = append(remotePackages, f)
				}
			}
		}
	}

	// 如果远程找到了，返回最新版本
	if len(remotePackages) > 0 {
		latest := findLatestVersion(remotePackages, re)
		return latest, nil
	}

	// 2. 在本地目录查找
	var localPackages []string
	for _, dir := range localDirs {
		matches, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("yashandb-*-linux-%s.tar.gz", arch)))
		if err == nil {
			for _, m := range matches {
				if re.MatchString(filepath.Base(m)) {
					localPackages = append(localPackages, m)
				}
			}
		}
	}

	if len(localPackages) == 0 {
		return "", fmt.Errorf("no yashandb package found for architecture %s in remote dir '%s' or local dirs %v", arch, remoteDir, localDirs)
	}

	// 返回最新版本的本地路径（文件名，后续会通过 FindAndDistribute 上传）
	latest := findLatestVersion(localPackages, re)
	return filepath.Base(latest), nil
}

// FindLatestYCMPackage 自动查找最新版本的 YCM 软件包
// 软件包格式: yashandb-cloud-manager-23.5.3.2-linux-x86_64.tar.gz 或 yashandb-cloud-manager-23.5.3.2-linux-aarch64.tar.gz
// 返回找到的软件包路径（远程或本地）
func FindLatestYCMPackage(
	executor runner.Executor,
	localDirs []string,
	remoteDir string,
) (string, error) {
	// 获取远程架构
	arch := "x86_64" // 默认
	result, _ := executor.Execute("uname -m", false)
	if result != nil && strings.TrimSpace(result.GetStdout()) != "" {
		remoteArch := strings.TrimSpace(result.GetStdout())
		if remoteArch == "aarch64" || remoteArch == "arm64" {
			arch = "aarch64"
		}
	}

	pattern := fmt.Sprintf(`yashandb-cloud-manager-(\d+\.\d+\.\d+\.\d+)-linux-%s\.tar\.gz`, arch)
	re := regexp.MustCompile(pattern)

	// 1. 先在远程目录查找
	var remotePackages []string
	if remoteDir != "" {
		result, _ := executor.Execute(fmt.Sprintf("ls -1 %s/yashandb-cloud-manager-*-linux-%s.tar.gz 2>/dev/null || true", remoteDir, arch), false)
		if result != nil && result.GetStdout() != "" {
			files := strings.Split(strings.TrimSpace(result.GetStdout()), "\n")
			for _, f := range files {
				f = strings.TrimSpace(f)
				if f != "" && re.MatchString(filepath.Base(f)) {
					remotePackages = append(remotePackages, f)
				}
			}
		}
	}

	// 如果远程找到了，返回最新版本
	if len(remotePackages) > 0 {
		latest := findLatestVersion(remotePackages, re)
		return latest, nil
	}

	// 2. 在本地目录查找
	var localPackages []string
	for _, dir := range localDirs {
		matches, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("yashandb-cloud-manager-*-linux-%s.tar.gz", arch)))
		if err == nil {
			for _, m := range matches {
				if re.MatchString(filepath.Base(m)) {
					localPackages = append(localPackages, m)
				}
			}
		}
	}

	if len(localPackages) == 0 {
		return "", fmt.Errorf("no yashandb-cloud-manager package found for architecture %s in remote dir '%s' or local dirs %v", arch, remoteDir, localDirs)
	}

	// 返回最新版本的本地路径（文件名，后续会通过 FindAndDistribute 上传）
	latest := findLatestVersion(localPackages, re)
	return filepath.Base(latest), nil
}

// FindLatestYMPPackage 自动查找最新版本的 YMP 软件包
// 软件包格式: yashan-migrate-platform-23.5.3.2-linux-x86_64.zip 或 yashan-migrate-platform-23.5.3.2-linux-aarch64.zip
// 返回找到的软件包路径（远程或本地）
func FindLatestYMPPackage(
	executor runner.Executor,
	localDirs []string,
	remoteDir string,
) (string, error) {
	// 获取远程架构
	arch := "x86_64" // 默认
	result, _ := executor.Execute("uname -m", false)
	if result != nil && strings.TrimSpace(result.GetStdout()) != "" {
		remoteArch := strings.TrimSpace(result.GetStdout())
		if remoteArch == "aarch64" || remoteArch == "arm64" {
			arch = "aarch64"
		}
	}

	pattern := fmt.Sprintf(`yashan-migrate-platform-(\d+\.\d+\.\d+\.\d+)-linux-%s\.zip`, arch)
	re := regexp.MustCompile(pattern)

	// 1. 先在远程目录查找
	var remotePackages []string
	if remoteDir != "" {
		result, _ := executor.Execute(fmt.Sprintf("ls -1 %s/yashan-migrate-platform-*-linux-%s.zip 2>/dev/null || true", remoteDir, arch), false)
		if result != nil && result.GetStdout() != "" {
			files := strings.Split(strings.TrimSpace(result.GetStdout()), "\n")
			for _, f := range files {
				f = strings.TrimSpace(f)
				if f != "" && re.MatchString(filepath.Base(f)) {
					remotePackages = append(remotePackages, f)
				}
			}
		}
	}

	// 如果远程找到了，返回最新版本
	if len(remotePackages) > 0 {
		latest := findLatestVersion(remotePackages, re)
		return latest, nil
	}

	// 2. 在本地目录查找
	var localPackages []string
	for _, dir := range localDirs {
		matches, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("yashan-migrate-platform-*-linux-%s.zip", arch)))
		if err == nil {
			for _, m := range matches {
				if re.MatchString(filepath.Base(m)) {
					localPackages = append(localPackages, m)
				}
			}
		}
	}

	if len(localPackages) == 0 {
		return "", fmt.Errorf("no yashan-migrate-platform package found for architecture %s in remote dir '%s' or local dirs %v", arch, remoteDir, localDirs)
	}

	// 返回最新版本的本地路径（文件名，后续会通过 FindAndDistribute 上传）
	latest := findLatestVersion(localPackages, re)
	return filepath.Base(latest), nil
}

// findLatestVersion 从文件列表中找到版本号最大的文件
func findLatestVersion(files []string, re *regexp.Regexp) string {
	if len(files) == 0 {
		return ""
	}
	if len(files) == 1 {
		return files[0]
	}

	// 提取版本号并排序
	type versionFile struct {
		file    string
		version []int
	}

	var versionFiles []versionFile
	for _, f := range files {
		matches := re.FindStringSubmatch(filepath.Base(f))
		if len(matches) > 1 {
			versionStr := matches[1]
			parts := strings.Split(versionStr, ".")
			version := make([]int, len(parts))
			for i, p := range parts {
				v, _ := strconv.Atoi(p)
				version[i] = v
			}
			versionFiles = append(versionFiles, versionFile{file: f, version: version})
		}
	}

	if len(versionFiles) == 0 {
		return files[0]
	}

	// 按版本号降序排序
	sort.Slice(versionFiles, func(i, j int) bool {
		vi, vj := versionFiles[i].version, versionFiles[j].version
		for k := 0; k < len(vi) && k < len(vj); k++ {
			if vi[k] != vj[k] {
				return vi[k] > vj[k]
			}
		}
		return len(vi) > len(vj)
	})

	return versionFiles[0].file
}
