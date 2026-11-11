package model

import (
	"reflect"
	"testing"
)

func TestTaskOrigin(t *testing.T) {
	group := &JobOrigin{RunID: "original-run", JobID: "original-array"}
	task := &JobOrigin{RunID: "other-run", JobID: "specific-task", AttemptID: "specific-attempt"}
	command := QueuedCommand{
		ID: "copy-array", Array: &ArraySpec{First: 1, Last: 4, Tasks: []int{1, 3, 4}},
		Origin: group, TaskOrigins: map[string]*JobOrigin{"copy-array-3": task, "copy-array-4": nil},
	}
	for _, test := range []struct {
		name   string
		task   int
		origin *JobOrigin
		want   *JobOrigin
	}{
		{"inherited", 1, group, &JobOrigin{RunID: group.RunID, JobID: "original-array-1"}},
		{"explicit", 3, group, task},
		{"nil task origin inherits", 4, group, &JobOrigin{RunID: group.RunID, JobID: "original-array-4"}},
		{"no origin", 1, nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			command.Origin = test.origin
			if got := TaskOrigin(command, test.task); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("TaskOrigin(%d)=%+v want %+v", test.task, got, test.want)
			}
			if group.JobID != "original-array" {
				t.Fatal("changed array origin while resolving a task")
			}
		})
	}
}

func TestOriginOfArrayTasks(t *testing.T) {
	arrayOrigin := &JobOrigin{RunID: "source", JobID: "src", Status: "failed"}
	own := &JobOrigin{RunID: "other", JobID: "x-2", AttemptID: "att"}
	plain := &JobOrigin{RunID: "source", JobID: "plain"}
	queue := Queue{Commands: []QueuedCommand{
		{ID: "x", Array: &ArraySpec{First: 1, Last: 3}, Origin: arrayOrigin, TaskOrigins: map[string]*JobOrigin{"x-2": own, "x-3": nil}},
		{ID: "y", Array: &ArraySpec{Tasks: []int{5}}},
		{ID: "p", Origin: plain},
	}}
	narrowed := func(task string) *JobOrigin {
		origin := *arrayOrigin
		origin.JobID = "src-" + task
		return &origin
	}
	for jobID, want := range map[string]*JobOrigin{
		"x":    arrayOrigin,
		"x-1":  narrowed("1"),
		"x-2":  own,
		"x-3":  narrowed("3"),
		"x-4":  nil, // not a task of the array
		"x-01": nil,
		"x-":   nil,
		"y-5":  nil, // an array without an origin
		"p":    plain,
		"q":    nil,
	} {
		if got := queue.OriginOf(jobID); !reflect.DeepEqual(got, want) {
			t.Errorf("OriginOf(%q) = %+v, want %+v", jobID, got, want)
		}
	}
	want := map[string]*JobOrigin{"x": arrayOrigin, "x-1": narrowed("1"), "x-2": own, "x-3": narrowed("3"), "p": plain}
	if got := QueueOriginsByJobID(queue); !reflect.DeepEqual(got, want) {
		t.Errorf("QueueOriginsByJobID = %+v, want %+v", got, want)
	}
}
