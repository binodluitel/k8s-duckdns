package utils

import (
	"fmt"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeRecorder is used as a fake event recorder during tests.
// It is thread safe. It is usable when created manually and not by FakeRecorder;
// however, all events may be thrown away in this case.
type FakeRecorder struct {
	bufferSize int
	*sync.Mutex
	Events        map[string]chan string
	IncludeObject bool
}

// NewFakeRecorder creates a fake event recorder with an event channel with buffer of given size.
func NewFakeRecorder(bufferSize int) *FakeRecorder {
	return &FakeRecorder{
		Mutex:      &sync.Mutex{},
		bufferSize: bufferSize,
		Events:     make(map[string]chan string),
	}
}

func (f FakeRecorder) Event(object runtime.Object, eventType, reason, message string) {
	cliObject, ok := object.(client.Object)
	if !ok {
		return
	}
	f.generateEvent(
		cliObject.GetNamespace()) <- fmt.Sprintf(
		"%s %s %s%s",
		eventType,
		reason,
		message,
		f.objectToString(object, f.IncludeObject),
	)
}

func (f FakeRecorder) Eventf(
	object runtime.Object,
	eventType, reason,
	messageFmt string,
	args ...interface{},
) {
	cliObject, ok := object.(client.Object)
	if !ok {
		// Skip non-client objects
		return
	}
	f.generateEvent(
		cliObject.GetNamespace()) <- fmt.Sprintf(
		"%s %s %s%s",
		eventType,
		reason,
		fmt.Sprintf(messageFmt, args...),
		f.objectToString(object, f.IncludeObject))
}

func (f FakeRecorder) AnnotatedEventf(
	object runtime.Object,
	_ map[string]string,
	eventType,
	reason,
	messageFmt string,
	args ...interface{},
) {
	f.Eventf(object, eventType, reason, messageFmt, args...)
}

func (f FakeRecorder) generateEvent(namespace string) chan string {
	f.Lock()
	defer f.Unlock()
	event, ok := f.Events[namespace]
	if !ok {
		event = make(chan string, f.bufferSize)
		f.Events[namespace] = event
	}
	return event
}

func (f FakeRecorder) objectToString(object runtime.Object, includeObject bool) string {
	if !includeObject {
		return ""
	}
	return spew.Sdump(object)
}
