package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/juncaifeng/cache-migrator/pkg/model"
)

var stdin = bufio.NewReader(os.Stdin)

// readLine 读取一行输入
func readLine() string {
	s, _ := stdin.ReadString('\n')
	return strings.TrimSpace(s)
}

// Confirm 询问是否继续
func Confirm(message string) bool {
	fmt.Printf("%s [y/N]: ", message)
	s := readLine()
	return strings.EqualFold(s, "y") || strings.EqualFold(s, "yes")
}

// SelectCaches 让用户多选要迁移的缓存
func SelectCaches(caches []*model.Cache) []*model.Cache {
	if len(caches) == 0 {
		fmt.Println("未检测到可迁移的缓存。")
		return nil
	}

	fmt.Println("\n检测到的缓存：")
	for i, c := range caches {
		fmt.Printf("  [%2d] %-16s  %-12s  %s\n", i+1, c.DisplayName, c.SizeHuman(), c.CurrentPath)
	}
	fmt.Println("\n请输入要迁移的编号，多个用空格/逗号分隔，输入 0 全选，直接回车取消: ")

	input := readLine()
	if input == "" {
		return nil
	}
	if input == "0" {
		return caches
	}

	fields := strings.FieldsFunc(input, func(r rune) bool {
		return r == ' ' || r == ',' || r == '，'
	})

	selected := make(map[int]bool)
	for _, f := range fields {
		idx, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || idx < 1 || idx > len(caches) {
			fmt.Printf("忽略无效编号: %s\n", f)
			continue
		}
		selected[idx-1] = true
	}

	var out []*model.Cache
	for i, c := range caches {
		if selected[i] {
			out = append(out, c)
		}
	}
	return out
}
