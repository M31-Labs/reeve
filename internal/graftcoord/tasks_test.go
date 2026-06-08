package graftcoord

import "testing"

func TestParseTasksClassifiesManagedAndUnmanaged(t *testing.T) {
	body := []byte(`{
  "tasks": [
    {
      "id": "task-1",
      "title": "Fix build",
      "status": "pending",
      "description": "Do it\n\n` + "```reeve-task" + `\ndedup_key: abc\nspace_uri: hypha://m31labs/reeve\nsignal_kind: red-build\ntarget: ./...\nseverity: 0.9\ncreated_by: agent://reeve/conductor\nretry_count: 0\n` + "```" + `\n"
    },
    {"id": "task-2", "title": "Manual", "status": "pending", "description": "no trailer"},
    {"id": "task-2", "title": "Manual", "status": "pending", "description": "no trailer", "source_workspace": "duplicate"}
  ]
}`)
	tasks, err := ParseTasks(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len=%d", len(tasks))
	}
	if !tasks[0].Managed || tasks[0].Blocked {
		t.Fatalf("task 1 should be managed: %#v", tasks[0])
	}
	if tasks[1].Managed || !tasks[1].Blocked {
		t.Fatalf("task 2 should be unmanaged blocked: %#v", tasks[1])
	}
	counts := CountByStatus(tasks)
	if counts["pending"] != 2 || counts["unmanaged_blocked"] != 1 {
		t.Fatalf("counts=%#v", counts)
	}
}
