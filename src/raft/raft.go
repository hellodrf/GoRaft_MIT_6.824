package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"log"
	"math/rand"
	"sync"
	"time"
)
import "sync/atomic"
import "../labrpc"

// import "bytes"
// import "../labgob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// Persistent
	currentTerm int32
	votedFor    int32
	logs        []LogEntry
	state       int32 // -1 follower; 0 candidate; 1 leader

	commitIndex int
	lastApplied int

	nextIndex  []int32
	matchIndex []int32

	// Channels
	electionChan  chan int
	heartbeatChan chan int
}

type LogEntry struct {
	Term    int32
	Command string
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isLeader bool

	// Your code here (2A).
	term = int(atomic.LoadInt32(&rf.currentTerm))
	isLeader = atomic.LoadInt32(&rf.state) == 1

	return term, isLeader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int32
	CandidateId  int32
	LastLogIndex int
	LastLogTerm  int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int32
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term         int32
	LeaderId     int
	PrevLogIndex int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int32
	Success bool
}

func (rf *Raft) SendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	log.Println("Node", rf.me, ": Heartbeat from ", args.LeaderId)
	if args.Term < rf.currentTerm {
		reply.Success = false
		return
	}
	reply.Success = true
	if atomic.LoadInt32(&rf.state) == 1 {
		rf.heartbeatChan <- 0 // stop heartbeat
	}
	atomic.StoreInt32(&rf.currentTerm, reply.Term)
	atomic.StoreInt32(&rf.state, -1)
	rf.electionChan <- 1
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		reply.VoteGranted = false
		return
	}

	vt := atomic.LoadInt32(&rf.votedFor)
	if vt == -1 || vt == args.CandidateId {
		reply.VoteGranted = true
		return
	} else {
		reply.VoteGranted = false
		return
	}
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
	rf.electionChan <- -1
	rf.heartbeatChan <- -1
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) election(source string) {

	voteCount := int32(0)
	if !atomic.CompareAndSwapInt32(&rf.state, rf.state, 0) {
		// only competing process is *accepting a new leader*
		return
	}

	if rf.votedFor != -1 {
		return
	}

	log.Println("Node", rf.me, ": Election triggered by "+source)
	log.Println("Node", rf.me, ": voted myself!")
	atomic.StoreInt32(&rf.state, 0)
	atomic.StoreInt32(&rf.votedFor, int32(rf.me))
	voteCount = int32(1)
	voteFailed := int32(0)
	threshold := int32(len(rf.peers) / 2)

	for i := 0; i < len(rf.peers); i++ {
		if i != rf.me {
			args := &RequestVoteArgs{
				Term:         rf.currentTerm,
				CandidateId:  int32(rf.me),
				LastLogIndex: 0,
				LastLogTerm:  0,
			}
			reply := &RequestVoteReply{}
			if rf.sendRequestVote(i, args, reply) {
				if reply.VoteGranted {
					for !atomic.CompareAndSwapInt32(&voteCount, voteCount, voteCount+1) {
					}
					log.Println("Node", rf.me, ": vote received from node", i)
					if voteCount > threshold && atomic.LoadInt32(&rf.dead) == 0 {
						// we won!
						log.Println("Node", rf.me, "won election term", rf.currentTerm)
						atomic.StoreInt32(&rf.state, 1)
						rf.heartbeatChan <- 1
						return
					}
				} else {
					voteFailed += 1
					log.Println("Node", rf.me, ": devote received from node", i)
				}
			} else {
				log.Println("Failed to send requestVote RPC to peer", i)
			}
		}

		if atomic.LoadInt32(&rf.state) != 0 {
			return
		}
		if rf.killed() {
			log.Println("Raft node is dead, election goroutine returned.")
			return
		}
	}
	if atomic.LoadInt32(&rf.state) != 0 {
		return
	}

	for voteFailed+voteCount != int32(len(rf.peers)) {
		time.Sleep(20 * time.Millisecond)
	}

	if voteFailed >= threshold && atomic.LoadInt32(&rf.dead) == 0 {
		log.Println("Node", rf.me, ": No one won!")
		time.Sleep(time.Duration(rand.Intn(200-100) + 100))
		//rf.election("failed election")
	}
}

func (rf *Raft) heartbeat() {
	args := AppendEntriesArgs{
		Term:         atomic.LoadInt32(&rf.currentTerm),
		LeaderId:     rf.me,
		PrevLogIndex: 0,
		Entries:      nil,
		LeaderCommit: 0,
	}
	reply := AppendEntriesReply{}

	for i := 0; i < len(rf.peers); i++ {
		if i != rf.me {
			rf.SendAppendEntries(i, &args, &reply)
			log.Println("Node", rf.me, ": Heartbeat sent to ", i)
		}
	}
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.state = 0
	rf.currentTerm = -1
	rf.votedFor = -1
	rf.logs = make([]LogEntry, 1000)
	rf.commitIndex = -1
	rf.lastApplied = -1

	rf.electionChan = make(chan int)
	rf.heartbeatChan = make(chan int)

	go func() {
		rand.Seed(time.Now().UnixNano())
		s := time.Duration(rand.Intn(550-450) + 450) // 450ms - 849ms
		time.Sleep(time.Duration(rand.Intn(3000)))

		rf.election("fresh startup")

		select {
		case o := <-rf.electionChan:
			if o < 0 {
				log.Println("Election goroutine received stop signal ", o)
				return
			}
		case <-time.After(s * time.Millisecond):
			if atomic.LoadInt32(&rf.state) < 0 {
				rf.election("timeout")
			}
		}
	}()

	go func() {
		for {
			i := <-rf.heartbeatChan
			switch i {
			case -1:
				log.Println("Heartbeat goroutine received stop signal ", i)
				return
			case 1:
				select {
				case <-time.After(110 * time.Millisecond):
					rf.heartbeat()
				case s := <-rf.heartbeatChan:
					if s == -1 {
						log.Println("Heartbeat goroutine received stop signal ", s)
						return
					}
					if s == 0 {
						break
					}
				}
			}
		}
	}()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}
