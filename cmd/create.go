/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"
)

//go:embed templates/*
var embeddedTemplates embed.FS

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "init",
	Short: "根据预设的模板创建一个新项目",
	Long:  `此命令会引导您输入必要信息，然后根据 'templates' 目录下的蓝图生成一个完整的项目结构。`,
	Run: func(cmd *cobra.Command, args []string) {
		// 监听 Ctrl+C 信号
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			_ = <-sigChan
			log.Printf(yellow + "[WARN] 接收到退出信号，中止执行。" + reset)
			os.Exit(0)
		}()

		// 检查当前目录
		if _, err := os.Getwd(); err != nil {
			logFatalf("当前目录无效...")
		}

		// 获取用户输入
		var (
			projectName string
			needManyDir string
			subDirInput string
			subDirs     []string
			isMulti     bool
		)
		if err := survey.AskOne(&survey.Input{
			Message: "请输入项目名称:",
		}, &projectName,
			survey.WithValidator(
				survey.ComposeValidators(
					survey.Required,
					func(ans interface{}) error {
						name, _ := ans.(string)
						if str := firstIllegalChar(name); str != "" {
							return fmt.Errorf(fmt.Sprintf("项目名称不能包含中文字符或特殊字符：%s", str))
						}

						// 检查项目目录是否存在
						if _, err := os.Stat(name); !os.IsNotExist(err) {
							return fmt.Errorf("该项目目录已存在")
						}
						return nil
					},
				),
			),
		); err == terminal.InterruptErr {
			log.Printf(yellow + "[WARN] 接收到退出信号，中止执行。" + reset)
			return
		}

		if err := survey.AskOne(&survey.Select{
			Message: "是否需要多个子目录？",
			Options: []string{"true", "false"},
			Default: "true",
			Help:    "是指是否需要创建多个子目录，如 `admin`, `app` 这种多端 API 在同一个项目中",
		}, &needManyDir); err == terminal.InterruptErr {
			log.Printf(yellow + "[WARN] 接收到退出信号，中止执行。" + reset)
			return
		}
		isMulti = needManyDir == "true"

		if isMulti {
			if err := survey.AskOne(&survey.Input{
				Message: "请输入子目录名称:（用中横线-分割，默认 admin-client）",
				Default: "admin-client",
			}, &subDirInput,
				survey.WithValidator(
					survey.ComposeValidators(
						survey.Required,
						func(ans interface{}) error {
							name, _ := ans.(string)
							nameArr := strings.Split(name, "-")
							if len(nameArr) < 1 {
								return fmt.Errorf("子目录名称格式错误，请使用中横线-分割")
							}

							for _, s := range nameArr {
								if str := firstIllegalChar(s); str != "" {
									return fmt.Errorf(fmt.Sprintf("子目录名称不能包含中文字符或特殊字符：%s", str))
								}
							}

							return nil
						},
					),
				),
			); err == terminal.InterruptErr {
				log.Printf(yellow + "[WARN] 接收到退出信号，中止执行。" + reset)
				return
			}
			subDirs = strings.Split(subDirInput, "-")
		}

		// 创建项目目录
		projectName = strings.TrimSpace(projectName)
		if err := os.MkdirAll(projectName, 0755); err != nil {
			logFatalf("创建目录失败...")
		}
		logInfo(fmt.Sprintf("已创建项目：%s", projectName))

		var err error
		if err = createEmptyDir(filepath.Join(projectName, "build", "scripts")); err != nil {
			logFatalf("创建 build/scripts 目录失败...")
		}
		if err = createEmptyDir(filepath.Join(projectName, "build", "docker")); err != nil {
			logFatalf("创建 build/docker 目录失败...")
		}
		if err = createEmptyDir(filepath.Join(projectName, "test")); err != nil {
			logFatalf("创建 test 目录失败...")
		}
		if err = createEmptyDir(filepath.Join(projectName, "pkg", "models")); err != nil {
			logFatalf("创建 pkg/model 目录失败...")
		}
		if err = createEmptyDir(filepath.Join(projectName, "static", "storage")); err != nil {
			logFatalf("创建 static/storage 目录失败...")
		}

		err = fs.WalkDir(embeddedTemplates, "templates", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}

			data, err := embeddedTemplates.ReadFile(path)
			if err != nil {
				return err
			}

			baseName := filepath.Base(path)
			switch baseName {
			case "user.api":
				if isMulti {
					for _, dir := range subDirs {
						targetPath := filepath.Join(projectName, "api", dir, baseName)
						if err := writeFile(targetPath, data); err != nil {
							logFatalf(fmt.Errorf("写入文件 %s 失败... %v", targetPath, err).Error())
						}
					}
				} else {
					targetPath := filepath.Join(projectName, "api", baseName)
					if err = writeFile(targetPath, data); err != nil {
						logFatalf(fmt.Errorf("写入文件 %s 失败... %v", targetPath, err).Error())
					}
				}
			case "config.yaml":
				if isMulti {
					for _, dir := range subDirs {
						name := fmt.Sprintf("%s.yaml", dir)
						targetPath := filepath.Join(projectName, "configs", name)
						if err = writeFile(targetPath, data); err != nil {
							logFatalf(fmt.Errorf("写入文件 %s 失败... %v", targetPath, err).Error())
						}
					}
				} else {
					targetPath := filepath.Join(projectName, "configs", baseName)
					if err = writeFile(targetPath, data); err != nil {
						logFatalf(fmt.Errorf("写入文件 %s 失败... %v", targetPath, err).Error())
					}
				}
			case "env":
				port := 18168
				if isMulti {
					for _, dir := range subDirs {
						targetPath := filepath.Join(projectName, fmt.Sprintf(".env.%s", dir))
						err = renderTemplateFile(path, targetPath, map[string]string{
							"projectName":         projectName + "-" + dir,
							"projectPort":         fmt.Sprintf(":%d", port),
							"projectRouterPrefix": "api/v1",
						})
						port++
					}
				} else {
					targetPath := filepath.Join(projectName, "."+baseName)
					err = renderTemplateFile(path, targetPath, map[string]string{
						"projectName":         projectName,
						"projectPort":         fmt.Sprintf(":%d", port),
						"projectRouterPrefix": "api/v1",
					})
				}
			case "gitignore":
				err = writeFile(filepath.Join(projectName, ".gitignore"), data)
			case "README1.md":
				if !isMulti {
					err = writeFile(filepath.Join(projectName, "README.md"), data)
				}
			case "README2.md":
				if isMulti {
					err = writeFile(filepath.Join(projectName, "README.md"), data)
				}
			case "gen.sh":
				err = writeFile(filepath.Join(projectName, "build", "scripts", "gen.sh"), data)
			case "gen.sh.template":
				if isMulti {
					for _, dir := range subDirs {
						targetPath := filepath.Join(projectName, "build", "scripts", fmt.Sprintf("gen_%s.sh", dir))
						err = renderTemplateFile(path, targetPath, map[string]string{
							"Name":       dir,
							"MainPath":   fmt.Sprintf("./cmd/%s/main.go", dir),
							"ConfigPath": fmt.Sprintf("./configs/%s.yaml", dir),
							"EnvPath":    fmt.Sprintf(".env.%s", dir),
							"SrcPath":    "./api",
							"OutputPath": "./internal",
						})
					}
				} else {
					targetPath := filepath.Join(projectName, "build", "scripts", "gen_server.sh")
					err = renderTemplateFile(path, targetPath, map[string]string{
						"Name":       "server",
						"MainPath":   "./cmd/server/main.go",
						"ConfigPath": "./configs/config.yaml",
						"EnvPath":    ".env",
						"SrcPath":    "./api",
						"OutputPath": "./internal",
					})
				}
			case "start.sh.template":
				if isMulti {
					for _, dir := range subDirs {
						targetPath := filepath.Join(projectName, "build", "scripts", fmt.Sprintf("start_%s.sh", dir))
						err = renderTemplateFile(path, targetPath, map[string]string{
							"Name":       dir,
							"MainPath":   fmt.Sprintf("./cmd/%s/main.go", dir),
							"ConfigPath": fmt.Sprintf("./configs/%s.yaml", dir),
							"EnvPath":    fmt.Sprintf(".env.%s", dir),
						})
					}
				} else {
					targetPath := filepath.Join(projectName, "build", "scripts", "start_server.sh")
					err = renderTemplateFile(path, targetPath, map[string]string{
						"Name":       "server",
						"MainPath":   "./cmd/server/main.go",
						"ConfigPath": "./configs/config.yaml",
						"EnvPath":    ".env",
					})
				}
			}

			return err
		})

		if err != nil {
			logFatalf("复制模板失败...")
		}

		logInfo("基础目录结构创建完成")

		// 执行 go mod init
		if err := runCommandInDirNoOutput(projectName, "go", "mod", "init", projectName); err != nil {
			logFatalf("执行 go mod init 失败...")
		}
		logInfo("go mod 初始化完成")

		// 拉取依赖
		if err := runCommandInDirNoOutput(projectName, "go", "get", "-u", "github.com/soryetong/gooze-starter"); err != nil {
			logFatalf("拉取最新依赖失败...")
		}

		if isMulti {
			for _, dir := range subDirs {
				_, _ = handlerMain(filepath.Join(projectName, "cmd", dir), projectName, dir)
			}
		} else {
			_, _ = handlerMain(filepath.Join(projectName, "cmd", "server"), projectName, "")
		}

		var startTarget string
		var envName string
		var configName string
		if needManyDir == "true" {
			err := survey.AskOne(&survey.Select{
				Message: "初始化完成，是否现在启动某个服务？",
				Options: append(subDirs, "No"),
			}, &startTarget)
			if err == nil && startTarget != "No" {
				envName = fmt.Sprintf(".env.%s", startTarget)
				configName = fmt.Sprintf("%s.yaml", startTarget)
			}
		} else {
			var startNow bool
			_ = survey.AskOne(&survey.Confirm{
				Message: "初始化完成，是否现在启动服务？",
				Default: true,
			}, &startNow)
			if startNow {
				startTarget = "server"
				envName = ".env"
				configName = "config.yaml"
			}
		}

		if startTarget != "" && startTarget != "No" {
			logInfo("🚀  项目已启动")
			if err := runCommandInDir(projectName,
				"go", "run", "./"+filepath.Join("cmd", startTarget, "main.go"),
				"--config=./configs/"+configName,
				"--env="+envName,
			); err != nil {
				logFatalf("项目启动失败...")
			}
		}
	},
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}

func createEmptyDir(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(path, ".gitkeep"), []byte{}, 0644)
}

func init() {
	rootCmd.AddCommand(createCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func handlerMain(target, projectName, moduleName string) (string, string) {
	if err := genMain(target, ""); err != nil {
		logFatalf(fmt.Errorf("生成主入口文件失败... %v", err).Error())
	}

	if err := runCommandInDir(target, "go", "mod", "tidy"); err != nil {
		logFatalf(fmt.Errorf("拉取最新依赖失败... %v", err).Error())
	}

	logInfo(moduleName + "拉取 github.com/soryetong/gooze-starter 成功")

	configName := moduleName
	env := ".env." + moduleName
	if moduleName == "" {
		configName = "config"
		env = ".env"
	}
	configPath := filepath.Join("configs", configName+".yaml")
	mainPath := filepath.Join("cmd", moduleName, "main.go")
	// 自动生成代码
	if err := runCommandInDir(projectName,
		"go", "run", "./"+mainPath, "gen", "api",
		"--config=./"+configPath,
		"--env="+env,
		"--src=./api",
		"--output=./internal",
		"--log=false",
	); err != nil {
		logFatalf("自动生成代码失败...")
	}

	// 修改 主入口 文件
	serverPath := projectName + "/internal/bootstrap"
	if moduleName != "" {
		serverPath = fmt.Sprintf("%s/internal/%s/bootstrap", projectName, moduleName)
	}
	if err := genMain(target, serverPath); err != nil {
		logFatalf("更新主入口文件失败...")
	}
	if err := runCommandInDirNoOutput(projectName, "go", "mod", "tidy"); err != nil {
		logFatalf("拉取最新依赖失败...")
	}

	return mainPath, configPath
}
