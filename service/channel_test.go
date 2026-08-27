package service

import "testing"

func TestEnqueueEmailSkipsWhenDisabled(t *testing.T) {
	oldUseEmail := useEmail
	oldEmailChannel := emailChannel
	t.Cleanup(func() {
		useEmail = oldUseEmail
		emailChannel = oldEmailChannel
	})

	useEmail = false
	emailChannel = nil
	if ok := enqueueEmail(&EMailInfo{Title: "test"}); ok {
		t.Fatal("enqueueEmail returned true when email is disabled")
	}
}

func TestEnqueueEmailReturnsFalseWhenChannelIsFull(t *testing.T) {
	oldUseEmail := useEmail
	oldEmailChannel := emailChannel
	t.Cleanup(func() {
		useEmail = oldUseEmail
		emailChannel = oldEmailChannel
	})

	useEmail = true
	emailChannel = make(chan *EMailInfo, 1)
	emailChannel <- &EMailInfo{Title: "first"}

	if ok := enqueueEmail(&EMailInfo{Title: "second"}); ok {
		t.Fatal("enqueueEmail returned true for a full channel")
	}
}

func TestEnqueueLiveRecordReturnsErrorWhenWatcherNotStarted(t *testing.T) {
	oldChannel := liveRecordChannel
	t.Cleanup(func() {
		liveRecordChannel = oldChannel
	})

	liveRecordChannel = nil
	if err := enqueueLiveRecord(&NsyLiveInfo{Name: "test"}); err == nil {
		t.Fatal("enqueueLiveRecord returned nil error for nil channel")
	}
}

func TestEnqueueLiveRecordReturnsErrorWhenChannelIsFull(t *testing.T) {
	oldChannel := liveRecordChannel
	t.Cleanup(func() {
		liveRecordChannel = oldChannel
	})

	liveRecordChannel = make(chan *NsyLiveInfo, 1)
	liveRecordChannel <- &NsyLiveInfo{Name: "first"}

	if err := enqueueLiveRecord(&NsyLiveInfo{Name: "second"}); err == nil {
		t.Fatal("enqueueLiveRecord returned nil error for full channel")
	}
}
