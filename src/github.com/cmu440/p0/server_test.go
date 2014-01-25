// Official tests for a MultiEchoServer implementation.

package p0

import (
	"bufio"
	"container/list"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"testing"
	"time"
)

const (
	defaultStartDelay = 500
	startServerTries  = 5
	largeMsgSize      = 512
)

// largeByteSlice is used to inflate the size of messages sent to slow reader clients
// (thus, making it more likely that the server will block on 'Write' and causing
// messages to be dropped).
var largeByteSlice = make([]byte, largeMsgSize)

type testSystem struct {
	test     *testing.T      // Pointer to the current test case being run.
	server   MultiEchoServer // The student's server implementation.
	hostport string          // The server's host/port address.
}

type testClient struct {
	id   int      // A unique id identifying this client.
	conn net.Conn // The client's TCP connection.
	slow bool     // True iff this client reads slowly.
}

type networkEvent struct {
	cli      *testClient // The client that received a network event.
	readMsg  string      // The message read from the network.
	writeMsg string      // The message to write to the network.
	err      error       // Notifies us that an error has occurred (if non-nil).
}

// countEvent is used to describe a round in Count tests.
type countEvent struct {
	start int // # of connections to start in a round.
	kill  int // # of connections to kill in a round.
	delay int // Wait time in ms before the next round begins.
}

func newTestSystem(t *testing.T) *testSystem {
	return &testSystem{test: t}
}

func newTestClients(num int, slow bool) []*testClient {
	clients := make([]*testClient, num)
	for i := range clients {
		clients[i] = &testClient{id: i, slow: slow}
	}
	return clients
}

// startServer attempts to start a server on a random port, retrying up to numTries
// times if necessary. A non-nil error is returned if the server failed to start.
func (ts *testSystem) startServer(numTries int) error {
	randGen := rand.New(rand.NewSource(time.Now().Unix()))
	var err error
	for i := 0; i < numTries; i++ {
		ts.server = New()
		if ts.server == nil {
			return errors.New("server returned by New() must not be nil")
		}
		port := 2000 + randGen.Intn(10000)
		if err := ts.server.Start(port); err == nil {
			ts.hostport = net.JoinHostPort("localhost", strconv.Itoa(port))
			return nil
		}
		fmt.Printf("Warning! Failed to start server on port %d: %s.\n", port, err)
		time.Sleep(time.Duration(50) * time.Millisecond)

	}
	if err != nil {
		return fmt.Errorf("failed to start server after %d tries", numTries)
	}
	return nil
}

// startClients connects the specified clients with the server, and returns a
// non-nil error if one or more of the clients failed to establish a connection.
func (ts *testSystem) startClients(clients ...*testClient) error {
	for _, cli := range clients {
		if addr, err := net.ResolveTCPAddr("tcp", ts.hostport); err != nil {
			return err
		} else if conn, err := net.DialTCP("tcp", nil, addr); err != nil {
			return err
		} else {
			cli.conn = conn
		}
	}
	return nil
}

// killClients kills the specified clients (i.e. closes their connections).
func (ts *testSystem) killClients(clients ...*testClient) {
	for i := range clients {
		cli := clients[i]
		if cli != nil && cli.conn != nil {
			cli.conn.Close()
		}
	}
}

// startReading signals the specified clients to begin reading from the network. Messages
// read over the network are sent back to the test runner through the readChan channel.
func (ts *testSystem) startReading(readChan chan<- *networkEvent, clients ...*testClient) {
	for _, cli := range clients {
		// Create new instance of cli for the goroutine
		// (see http://golang.org/doc/effective_go.html#channels).
		cli := cli
		go func() {
			reader := bufio.NewReader(cli.conn)
			for {
				// Read up to and including the first '\n' character.
				msgBytes, err := reader.ReadBytes('\n')
				if err != nil {
					readChan <- &networkEvent{err: err}
					return
				}
				// Notify the test runner that a message was read from the network.
				readChan <- &networkEvent{
					cli:     cli,
					readMsg: string(msgBytes),
				}
			}
		}()
	}
}

// startWriting signals the specified clients to begin writing to the network. In order
// to ensure that reads/writes are synchronized, messages are sent to and written in
// the main test runner event loop.
func (ts *testSystem) startWriting(writeChan chan<- *networkEvent, numMsgs int, clients ...*testClient) {
	for _, cli := range clients {
		// Create new instance of cli for the goroutine
		// (see http://golang.org/doc/effective_go.html#channels).
		cli := cli
		go func() {
			// Client and message IDs guarantee that msgs sent over the network are unique.
			for msgID := 0; msgID < numMsgs; msgID++ {
				// Notify the test runner that a message should be written to the network.
				writeChan <- &networkEvent{
					cli:      cli,
					writeMsg: fmt.Sprintf("%d %t %d\n", cli.id, cli.slow, msgID),
				}
				if msgID%100 == 0 {
					// Give readers some time to consume message before writing
					// the next batch of messages.
					time.Sleep(time.Duration(100) * time.Millisecond)
				}
			}
		}()
	}
}

func (ts *testSystem) runTest(numMsgs, timeout int, normalClients, slowClients []*testClient, slowDelay int) error {
	numNormalClients := len(normalClients)
	numSlowClients := len(slowClients)
	numClients := numNormalClients + numSlowClients
	hasSlowClients := numSlowClients != 0

	totalWrites := numMsgs * numClients
	totalReads := totalWrites * numClients
	normalReads, normalWrites := 0, 0
	slowReads, slowWrites := 0, 0

	msgMap := make(map[string]int)
	readChan, writeChan := make(chan *networkEvent), make(chan *networkEvent)

	// Begin reading in the background (one goroutine per client).
	fmt.Println("All clients beginning to write...")
	ts.startWriting(writeChan, numMsgs, normalClients...)
	if hasSlowClients {
		ts.startWriting(writeChan, numMsgs, slowClients...)
	}

	// Begin reading in the background (one goroutine per client).
	ts.startReading(readChan, normalClients...)
	if hasSlowClients {
		fmt.Println("Normal clients beginning to read...")
		time.AfterFunc(time.Duration(slowDelay)*time.Millisecond, func() {
			fmt.Println("Slow clients beginning to read...")
			ts.startReading(readChan, slowClients...)
		})
	} else {
		fmt.Println("All clients beginning to read...")
	}

	// Returns a channel that will be sent a notification if a timeout occurs.
	timeoutChan := time.After(time.Duration(timeout) * time.Millisecond)

	for slowReads+normalReads < totalReads || slowWrites+normalWrites < totalWrites {
		select {
		case cmd := <-readChan:
			cli, msg := cmd.cli, cmd.readMsg
			if cmd.err != nil {
				return cmd.err
			}
			if hasSlowClients {
				msg = string([]byte(msg)[largeMsgSize:])
			}
			if v, ok := msgMap[msg]; !ok {
				// Abort! Client received a message that was never sent.
				return fmt.Errorf("client read unexpected message: %s", msg)
			} else if v > 1 {
				// We expect the message to be read v - 1 more times.
				msgMap[msg] = v - 1
			} else {
				// The message has been read for the last time.
				// Erase it from memory.
				delete(msgMap, msg)
			}
			if hasSlowClients && cli.slow {
				slowReads++
			} else {
				normalReads++
			}
		case cmd := <-writeChan:
			// Synchronizing writes is necessary because messages are read
			// concurrently in a background goroutine.
			cli, msg := cmd.cli, cmd.writeMsg
			if hasSlowClients {
				// Send larger messages to increase the likelihood that
				// messages will be dropped due to slow clients.
				if _, err := cli.conn.Write(largeByteSlice); err != nil {
					return err
				}
			}
			msgMap[msg] = numClients
			if _, err := cli.conn.Write([]byte(msg)); err != nil {
				// Abort! Error writing to the network.
				return err
			}
			if hasSlowClients && cli.slow {
				slowWrites++
			} else {
				normalWrites++
			}
		case <-timeoutChan:
			if hasSlowClients {
				if normalReads != totalWrites*numNormalClients {
					// Make sure non-slow clients received all written messages.
					return fmt.Errorf("non-slow clients received %d messages, expected %d",
						normalReads, totalWrites*numNormalClients)
				}
				if slowReads < 100 {
					// Make sure the server buffered 100 messages for
					// slow-reading clients.
					return errors.New("slow-reading client read less than 100 messages")
				}
				return nil
			}
			// Otherwise, if there are no slow clients, then no messages
			// should be dropped and the test should NOT timeout.
			return errors.New("test timed out")
		}
	}

	if hasSlowClients {
		// If there are slow clients, then at least some messages
		// should have been dropped.
		return errors.New("no messages were dropped by slow clients")
	}

	return nil
}

func (ts *testSystem) checkCount(expected int) error {
	if count := ts.server.Count(); count != expected {
		return fmt.Errorf("Count returned an incorrect value (returned %d, expected %d)",
			count, expected)
	}
	return nil
}

func testBasic(t *testing.T, name string, numClients, numMessages, timeout int) {
	fmt.Printf("========== %s: %d client(s), %d messages each ==========\n", name, numClients, numMessages)

	ts := newTestSystem(t)
	if err := ts.startServer(startServerTries); err != nil {
		t.Errorf("Failed to start server: %s\n", err)
		return
	}
	defer ts.server.Close()

	if err := ts.checkCount(0); err != nil {
		t.Error(err)
		return
	}

	allClients := newTestClients(numClients, false)
	if err := ts.startClients(allClients...); err != nil {
		t.Errorf("Failed to start clients: %s\n", err)
		return
	}
	defer ts.killClients(allClients...)

	// Give the server some time to register the clients before running the test.
	time.Sleep(time.Duration(defaultStartDelay) * time.Millisecond)

	if err := ts.checkCount(numClients); err != nil {
		t.Error(err)
		return
	}

	if err := ts.runTest(numMessages, timeout, allClients, []*testClient{}, 0); err != nil {
		t.Error(err)
		return
	}
}

func testSlowClient(t *testing.T, name string, numMessages, numSlowClients, numNormalClients, slowDelay, timeout int) {
	fmt.Printf("========== %s: %d total clients, %d slow client(s), %d messages each ==========\n",
		name, numSlowClients+numNormalClients, numSlowClients, numMessages)

	ts := newTestSystem(t)
	if err := ts.startServer(startServerTries); err != nil {
		t.Errorf("Failed to start server: %s\n", err)
		return
	}
	defer ts.server.Close()

	slowClients := newTestClients(numSlowClients, true)
	if err := ts.startClients(slowClients...); err != nil {
		t.Errorf("Test failed: %s\n", err)
		return
	}
	defer ts.killClients(slowClients...)
	normalClients := newTestClients(numNormalClients, false)
	if err := ts.startClients(normalClients...); err != nil {
		t.Errorf("Test failed: %s\n", err)
		return
	}
	defer ts.killClients(normalClients...)

	// Give the server some time to register the clients before running the test.
	time.Sleep(time.Duration(defaultStartDelay) * time.Millisecond)

	if err := ts.runTest(numMessages, timeout, normalClients, slowClients, slowDelay); err != nil {
		t.Error(err)
		return
	}
}

func (ts *testSystem) runCountTest(events []*countEvent, timeout int) {
	t := ts.test

	// Use a linked list to store a queue of clients.
	clients := list.New()
	defer func() {
		for clients.Front() != nil {
			ts.killClients(clients.Remove(clients.Front()).(*testClient))
		}
	}()

	// Count() should return 0 at the beginning of a count test.
	if err := ts.checkCount(0); err != nil {
		t.Error(err)
		return
	}

	timeoutChan := time.After(time.Duration(timeout) * time.Millisecond)

	for i, e := range events {
		fmt.Printf("Round %d: starting %d, killing %d\n", i+1, e.start, e.kill)
		for i := 0; i < e.start; i++ {
			// Create and start a new client, and add it to our
			// queue of clients.
			cli := new(testClient)
			if err := ts.startClients(cli); err != nil {
				t.Errorf("Failed to start clients: %s\n", err)
				return
			}
			clients.PushBack(cli)
		}
		for i := 0; i < e.kill; i++ {
			// Close the client's TCP connection with the server and
			// and remove it from the queue.
			cli := clients.Remove(clients.Front()).(*testClient)
			ts.killClients(cli)
		}
		select {
		case <-time.After(time.Duration(e.delay) * time.Millisecond):
			// Continue to the next round...
		case <-timeoutChan:
			t.Errorf("Test timed out.")
			return
		}
		if err := ts.checkCount(clients.Len()); err != nil {
			t.Errorf("Test failed during the event #%d: %s\n", i+1, err)
			return
		}
	}
}

func testCount(t *testing.T, name string, timeout int, max int, events ...*countEvent) {
	fmt.Printf("========== %s: %d rounds, up to %d clients started/killed per round ==========\n",
		name, len(events), max)

	ts := newTestSystem(t)
	if err := ts.startServer(startServerTries); err != nil {
		t.Errorf("Failed to start server: %s\n", err)
		return
	}
	defer ts.server.Close()

	// Give the server some time to register the clients before running the test.
	time.Sleep(time.Duration(defaultStartDelay) * time.Millisecond)

	ts.runCountTest(events, timeout)
}

func TestBasic1(t *testing.T) {
	testBasic(t, "TestBasic1", 1, 100, 5000)
}

func TestBasic2(t *testing.T) {
	testBasic(t, "TestBasic2", 1, 1500, 10000)
}

func TestBasic3(t *testing.T) {
	testBasic(t, "TestBasic3", 2, 20, 5000)
}

func TestBasic4(t *testing.T) {
	testBasic(t, "TestBasic4", 2, 1500, 10000)
}

func TestBasic5(t *testing.T) {
	testBasic(t, "TestBasic5", 5, 750, 10000)
}

func TestBasic6(t *testing.T) {
	testBasic(t, "TestBasic6", 10, 1500, 10000)
}

func TestCount1(t *testing.T) {
	testCount(t, "TestCount1", 5000, 20,
		&countEvent{start: 20, kill: 0, delay: 1000},
		&countEvent{start: 20, kill: 20, delay: 1000},
		&countEvent{start: 0, kill: 20, delay: 1000},
	)
}

func TestCount2(t *testing.T) {
	testCount(t, "TestCount2", 10000, 25,
		&countEvent{start: 5, kill: 0, delay: 1000},
		&countEvent{start: 15, kill: 5, delay: 1000},
		&countEvent{start: 25, kill: 15, delay: 1000},
		&countEvent{start: 15, kill: 25, delay: 1000},
		&countEvent{start: 0, kill: 15, delay: 1000},
	)
}

func TestSlowClient1(t *testing.T) {
	testSlowClient(t, "TestSlowClient1", 1000, 1, 1, 4000, 8000)
}

func TestSlowClient2(t *testing.T) {
	testSlowClient(t, "TestSlowClient2", 1000, 2, 2, 5000, 10000)
}
