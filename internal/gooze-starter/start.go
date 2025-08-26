/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package gooze_starter

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/soryetong/gooze-cli/pkg/util"
	"github.com/spf13/cobra"
)

//go:embed templates/*
var embeddedTemplates embed.FS

// StartCmd represents the create command
var StartCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate New Project from Templates",
	Long:  `此命令会引导您输入必要信息，然后根据 'templates' 目录下的蓝图生成一个完整的项目结构。`,
	Run: func(cmd *cobra.Command, args []string) {
		// 监听 Ctrl+C 信号
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			_ = <-sigChan
			util.LogWarn(" 接收到退出信号，中止执行。")
			os.Exit(0)
		}()

		// 检查当前目录
		if _, err := os.Getwd(); err != nil {
			util.LogFatalf("当前目录无效...")
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
						if str := util.FirstIllegalChar(name); str != "" {
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
			util.LogWarn(" 接收到退出信号，中止执行。")
			return
		}

		if err := survey.AskOne(&survey.Select{
			Message: "是否需要多个子目录？",
			Options: []string{"true", "false"},
			Default: "true",
			Help:    "是指是否需要创建多个子目录，如 `admin`, `app` 这种多端 API 在同一个项目中",
		}, &needManyDir); err == terminal.InterruptErr {
			util.LogWarn(" 接收到退出信号，中止执行。")
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
								if str := util.FirstIllegalChar(s); str != "" {
									return fmt.Errorf(fmt.Sprintf("子目录名称不能包含中文字符或特殊字符：%s", str))
								}
							}

							return nil
						},
					),
				),
			); err == terminal.InterruptErr {
				util.LogWarn(" 接收到退出信号，中止执行。")
				return
			}
			subDirs = strings.Split(subDirInput, "-")
		}

		// 创建项目目录
		projectName = strings.TrimSpace(projectName)
		if err := os.MkdirAll(projectName, 0755); err != nil {
			util.LogFatalf("创建目录失败...")
		}
		util.LogInfo(fmt.Sprintf("已创建项目：%s", projectName))

		var err error
		if err = createEmptyDir(filepath.Join(projectName, "build", "scripts")); err != nil {
			util.LogFatalf("创建 build/scripts 目录失败...")
		}
		if err = createEmptyDir(filepath.Join(projectName, "build", "docker")); err != nil {
			util.LogFatalf("创建 build/docker 目录失败...")
		}
		if err = createEmptyDir(filepath.Join(projectName, "test")); err != nil {
			util.LogFatalf("创建 test 目录失败...")
		}
		if err = createEmptyDir(filepath.Join(projectName, "models")); err != nil {
			util.LogFatalf("创建 models 目录失败...")
		}
		if err = createEmptyDir(filepath.Join(projectName, "static", "storage")); err != nil {
			util.LogFatalf("创建 static/storage 目录失败...")
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
							util.LogFatalf(fmt.Errorf("写入文件 %s 失败... %v", targetPath, err).Error())
						}
					}
				} else {
					targetPath := filepath.Join(projectName, "api", baseName)
					if err = writeFile(targetPath, data); err != nil {
						util.LogFatalf(fmt.Errorf("写入文件 %s 失败... %v", targetPath, err).Error())
					}
				}
			case "config.yaml":
				if isMulti {
					for _, dir := range subDirs {
						name := fmt.Sprintf("%s.yaml", dir)
						targetPath := filepath.Join(projectName, "configs", name)
						if err = writeFile(targetPath, data); err != nil {
							util.LogFatalf(fmt.Errorf("写入文件 %s 失败... %v", targetPath, err).Error())
						}
					}
				} else {
					targetPath := filepath.Join(projectName, "configs", baseName)
					if err = writeFile(targetPath, data); err != nil {
						util.LogFatalf(fmt.Errorf("写入文件 %s 失败... %v", targetPath, err).Error())
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
			case "rbac_model.conf":
				err = writeFile(filepath.Join(projectName, "configs", "rbac_model.conf"), data)
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
			util.LogFatalf("复制模板失败...")
		}

		util.LogInfo("基础目录结构创建完成")

		// 执行 go mod init
		if err := util.RunCommandInDirNoOutput(projectName, "go", "mod", "init", projectName); err != nil {
			util.LogFatalf("执行 go mod init 失败...")
		}
		util.LogInfo("go mod 初始化完成")

		// 拉取依赖
		if err := util.RunCommandInDirNoOutput(projectName, "go", "get", "-u", "github.com/soryetong/gooze-starter"); err != nil {
			util.LogFatalf("拉取最新依赖失败...")
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
			util.LogInfo("🚀  项目启动中......")
			if err := util.RunCommandInDir(projectName,
				"go", "run", "./"+filepath.Join("cmd", startTarget, "main.go"),
				"--config=./configs/"+configName,
				"--env="+envName,
			); err != nil {
				util.LogFatalf("项目启动失败...")
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

func handlerMain(target, projectName, moduleName string) (string, string) {
	if err := genMain(target, ""); err != nil {
		util.LogFatalf(fmt.Errorf("生成主入口文件失败... %v", err).Error())
	}

	if err := util.RunCommandInDir(target, "go", "mod", "tidy"); err != nil {
		util.LogFatalf(fmt.Errorf("拉取最新依赖失败... %v", err).Error())
	}

	util.LogInfo(moduleName + "拉取 github.com/soryetong/gooze-starter 成功")

	configName := moduleName
	env := ".env." + moduleName
	if moduleName == "" {
		configName = "config"
		env = ".env"
	}
	configPath := filepath.Join("configs", configName+".yaml")
	cmdDir := moduleName
	if cmdDir == "" {
		cmdDir = "server"
	}
	mainPath := filepath.Join("cmd", cmdDir, "main.go")
	// 自动生成代码
	if err := util.RunCommandInDir(projectName,
		"go", "run", "./"+mainPath, "gen", "api",
		"--config=./"+configPath,
		"--env="+env,
		"--src=./api",
		"--output=./internal",
		"--log=false",
	); err != nil {
		util.LogFatalf("自动生成代码失败...")
	}

	// 修改 主入口 文件
	serverPath := projectName + "/internal/bootstrap"
	if moduleName != "" {
		serverPath = fmt.Sprintf("%s/internal/%s/bootstrap", projectName, moduleName)
	}
	if err := genMain(target, serverPath); err != nil {
		util.LogFatalf("更新主入口文件失败...")
	}
	if err := util.RunCommandInDirNoOutput(projectName, "go", "mod", "tidy"); err != nil {
		util.LogFatalf("拉取最新依赖失败...")
	}

	return mainPath, configPath
}

func renderTemplateFile(srcPath, destPath string, data map[string]string) error {
	content, err := embeddedTemplates.ReadFile(srcPath)
	if err != nil {
		return err
	}
	tmpl, err := template.New(filepath.Base(srcPath)).Parse(string(content))
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	return tmpl.Execute(outFile, data)
}
