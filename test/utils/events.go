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
	events     map[string]chan string
	dumpObject bool
}

// NewFakeRecorder creates a fake event recorder with an event channel with a buffer of a given size.
func NewFakeRecorder(bufferSize int, dumpObject bool) *FakeRecorder {
	return &FakeRecorder{
		Mutex:      &sync.Mutex{},
		bufferSize: bufferSize,
		events:     make(map[string]chan string),
		dumpObject: dumpObject,
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
		f.objectToString(object),
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
		return
	}
	f.generateEvent(
		cliObject.GetNamespace()) <- fmt.Sprintf(
		"%s %s %s%s",
		eventType,
		reason,
		fmt.Sprintf(messageFmt, args...),
		f.objectToString(object))
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
	event, ok := f.events[namespace]
	if !ok {
		event = make(chan string, f.bufferSize)
		f.events[namespace] = event
	}
	return event
}

func (f FakeRecorder) objectToString(object runtime.Object) string {
	if !f.dumpObject {
		return ">>> Object not included in event"
	}
	return spew.Sdump(object)
}
