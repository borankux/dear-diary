package watcher

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/borankux/dear-diary/internal/process"
)

// TodoCompletionDetector 检测新日记中哪些 active todos 已完成。
type TodoCompletionDetector struct {
	store     *process.Store
	extractor *process.Extractor
}

// NewTodoCompletionDetector 创建 Todo 完成检测器。
func NewTodoCompletionDetector(store *process.Store) *TodoCompletionDetector {
	return &TodoCompletionDetector{
		store: store,
	}
}

// Detect 读取所有 active todos，将新日记内容 + todo 列表发给 AI，
// 返回应该标记为 done 的 todo ID 列表。
func (d *TodoCompletionDetector) Detect(newDiaryContent string) ([]int, error) {
	todos, err := d.store.ListActiveTodos()
	if err != nil {
		return nil, fmt.Errorf("读取 active todos 失败: %w", err)
	}
	if len(todos) == 0 {
		return nil, nil
	}

	// 复用 process.NewExtractor()
	extractor, err := process.NewExtractor()
	if err != nil {
		return nil, fmt.Errorf("创建 extractor 失败: %w", err)
	}
	d.extractor = extractor

	// 构建 todo 列表文本
	var todoList strings.Builder
	for _, todo := range todos {
		todoList.WriteString(fmt.Sprintf("ID %d: %s\n", todo.ID, todo.Text))
	}

	prompt := fmt.Sprintf(`你是一个待办完成检测助手。用户刚写了一篇新日记，以下是他当前所有 active 的待办事项：

%s

请阅读这篇日记，判断用户是否已经完成了哪些待办事项。只返回已完成的 todo ID 列表。

输出格式：
{"completed_ids": [1, 5, ...]}

如果没有提到任何完成事项，返回 {"completed_ids": []}
`, todoList.String())

	result, err := d.extractor.ExtractWithSystemPrompt(prompt, newDiaryContent)
	if err != nil {
		return nil, fmt.Errorf("AI 检测失败: %w", err)
	}

	// 解析 AI 返回的 JSON
	var completed struct {
		CompletedIDs []int `json:"completed_ids"`
	}
	if err := json.Unmarshal([]byte(result.RawJSON), &completed); err != nil {
		// 尝试从 items 中回退解析
		if len(result.Items) > 0 {
			for _, item := range result.Items {
				var id int
				if _, err := fmt.Sscanf(item.Title, "%d", &id); err == nil && id > 0 {
					completed.CompletedIDs = append(completed.CompletedIDs, id)
				}
			}
		}
		if len(completed.CompletedIDs) == 0 {
			return nil, fmt.Errorf("解析 AI 返回失败: %w", err)
		}
	}

	// 将对应的 todo 标记为 done
	for _, id := range completed.CompletedIDs {
		if err := d.store.SetTodoStatus(id, "done"); err != nil {
			log.Printf("【tododetector】标记 todo %d 为 done 失败: %v", id, err)
		} else {
			log.Printf("【tododetector】todo %d 已标记为完成", id)
		}
	}

	return completed.CompletedIDs, nil
}
