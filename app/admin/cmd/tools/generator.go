package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 表配置
type TableConfig struct {
	TableName   string
	StructName  string
	Description string
}

var (
	projectRoot string
	dsn         string
	tableList   string
)

func init() {
	flag.StringVar(&projectRoot, "project", "", "项目根目录路径")
	flag.StringVar(&dsn, "dsn", "", "数据库DSN连接字符串")
	flag.StringVar(&tableList, "tables", "", "要生成的表列表，逗号分隔")
}

func main() {
	flag.Parse()

	if dsn == "" {
		fmt.Println("错误: 必须指定数据库DSN连接字符串")
		fmt.Println("")
		fmt.Println("使用方法:")
		fmt.Println("  go run ./tools/generator.go -dsn \"root:passwd@tcp(localhost:3306)/kva?charset=utf8mb4&parseTime=True&loc=Local\"")
		fmt.Println("")
		flag.Usage()
		os.Exit(1)
	}

	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Printf("获取当前目录失败: %v\n", err)
			os.Exit(1)
		}
		projectRoot = cwd
	}

	// 连接数据库
	db, err := connectDB(dsn)
	if err != nil {
		fmt.Printf("连接数据库失败: %v\n", err)
		os.Exit(1)
	}

	// 创建输出目录
	outputDir := filepath.Join(projectRoot, "app/admin/internal/data/gen/dao")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("创建输出目录失败: %v\n", err)
		os.Exit(1)
	}

	// 创建生成器
	g := gen.NewGenerator(gen.Config{
		OutPath:      outputDir,
		WithUnitTest: false,
		ModelPkgPath: filepath.Join(projectRoot, "app/admin/internal/data/gen/model"),
	})

	g.UseDB(db)

	// 获取要生成的表配置
	allConfiguredTables := getAllTables()

	// 表选择逻辑：
	// 1. 如果指定了 -tables 参数，只生成指定的表
	// 2. 如果 getAllTables() 有配置表，只处理已配置的表
	// 3. 如果都没有配置，使用 DSN 中的全表
	var tables []TableConfig

	if tableList != "" {
		// 方式1: 使用 -tables 参数指定表
		requestedTables := strings.Split(tableList, ",")
		for _, t := range allConfiguredTables {
			for _, rt := range requestedTables {
				if strings.TrimSpace(rt) == t.TableName {
					tables = append(tables, t)
					break
				}
			}
		}
		if len(tables) == 0 {
			fmt.Println("警告: 指定的表不在配置列表中，请先在 getAllTables() 中添加配置")
			os.Exit(0)
		}
		fmt.Printf("使用 -tables 参数，生成 %d 张表\n", len(tables))
	} else if len(allConfiguredTables) > 0 {
		// 方式2: 使用 getAllTables() 中的配置表
		tables = allConfiguredTables
		fmt.Printf("使用 getAllTables() 配置，生成 %d 张表\n", len(tables))
	} else {
		// 方式3: 使用 DSN 中的全表
		fmt.Println("未配置任何表，将生成 DSN 中的所有表...")
		// 从 DSN 中提取数据库名: "root:root@tcp(localhost:3306)/jydb?..." -> "jydb"
		parts := strings.Split(dsn, "/")
		dbPart := parts[1]
		dbName := strings.Split(dbPart, "?")[0]
		var dbTables []string
		if err := db.Table("information_schema.tables").
			Where("table_schema = ?", dbName).
			Where("table_type = 'BASE TABLE'").
			Pluck("table_name", &dbTables).Error; err != nil {
			fmt.Printf("获取数据库表列表失败: %v\n", err)
			os.Exit(1)
		}
		for _, t := range dbTables {
			tables = append(tables, TableConfig{
				TableName:   t,
				StructName:  toCamelCase(t),
				Description: "",
			})
		}
		fmt.Printf("从数据库 [%s] 获取到 %d 张表\n", dbName, len(tables))
	}

	// 为每个表生成模型
	successCount := 0
	failCount := 0
	for _, tc := range tables {
		// 添加错误恢复机制
		func() {
			defer func() {
				if r := recover(); r != nil {
					failCount++
					fmt.Printf("ERROR: 表 %s 生成失败: %v\n", tc.TableName, r)
				}
			}()

			model := g.GenerateModelAs(tc.TableName, toCamelCase(tc.TableName))
			if model != nil {
				g.ApplyBasic(model)
				successCount++
				fmt.Printf("成功生成表 %s 的模型 -> %s\n", tc.TableName, tc.StructName)
			} else {
				failCount++
				fmt.Printf("跳过表 %s (表不存在或生成失败)\n", tc.TableName)
			}
		}()
	}

	// 执行生成
	g.Execute()
	fmt.Printf("\n代码生成完成! 成功: %d, 失败: %d\n", successCount, failCount)

	if failCount > 0 {
		fmt.Println("\n注意: 失败的表可能不存在于数据库中，请先初始化数据库")
		fmt.Println("  jydb 数据库表结构: 参考 docs/datong-jydb.sql")
	}
}

func connectDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库实例失败: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("测试数据库连接失败: %w", err)
	}

	return db, nil
}

func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, "")
}

// 所有表配置 (jydb 核心交易表)
func getAllTables() []TableConfig {
	tables := []TableConfig{}

	//tables = append(tables, TableConfig{TableName: "casbin_rule", StructName: "casbin", Description: "权限配置表"})
	//tables = append(tables, TableConfig{TableName: "jwt_blacklists", StructName: "blacklists", Description: "jwt黑名单"})

	return tables
}
