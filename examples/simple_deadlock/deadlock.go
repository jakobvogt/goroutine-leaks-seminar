package simple_deadlock

import "sync"

// SimpleDeadlock is a classic deadlock where all goroutines block.
// Runtime detects this and crashes with "fatal error: all goroutines are asleep - deadlock!"
func SimpleDeadlock() {
	ch := make(chan int)
	<-ch // blocks forever, no sender
}

// TwoGoroutineDeadlock shows a circular wait deadlock
func TwoGoroutineDeadlock() {
	var wg sync.WaitGroup
	ch1 := make(chan int)
	ch2 := make(chan int)

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-ch1
		ch2 <- 1
	}()

	go func() {
		defer wg.Done()
		<-ch2
		ch1 <- 1
	}()

	wg.Wait()
}

// SendWithoutReceiver: sending on unbuffered channel with no reciever
func SendWithoutReceiver() {
	ch := make(chan int)
	ch <- 42 // blocks forever
}
