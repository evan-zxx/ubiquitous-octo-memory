package paxos

import (
	"log"
	"time"
)

type acceptor struct {
	// server id
	id int
	// the number of the proposal this server will accept, or 0 if it has never received a Prepare request
	promiseNumber int
	// the number of the last proposal the server has accepted, or 0 if it never accepted any.
	acceptedNumber int
	// the value from the most recent proposal the server has accepted, or null if it has never accepted a proposal
	acceptedValue string

	learners []int
	net      network
}

func newAcceptor(id int, net network, learners ...int) *acceptor {
	return &acceptor{id: id, promiseNumber: 0, acceptedNumber: 0, acceptedValue: "", learners: learners, net: net}
}

func (a *acceptor) run() {
	log.Printf("acceptor %d run", a.id)
	for {
		msg, ok := a.net.recv(time.Hour)
		if !ok {
			continue
		}
		switch msg.tp {
		case Prepare:
			// 收到proposer的prepare包, 回复promise包
			promise, ok := a.handlePrepare(msg)
			if ok {
				a.net.send(promise)
			}
		case Propose:
			success := a.handleAccept(msg)
			if success {
				// 一旦accept成功 则通知learners, 而不是给proposer回包
				for _, l := range a.learners {
					msg := message{
						tp:     Accept,
						from:   a.id,
						to:     l,
						number: a.acceptedNumber,
						value:  a.acceptedValue,
					}
					a.net.send(msg)
				}
			}
		default:
			log.Panicf("acceptor: %d unexpected message type: %v", a.id, TpStr[msg.tp])
		}
	}
}

// Phase 1. (b) If an acceptor receives a prepare request with number n greater than that of
// any prepare request to which it has already responded, then it responds to the request
// with a promise not to accept any more proposals numbered less than n and with
// the highest-numbered proposal (if any) that it has accepted.
func (a *acceptor) handlePrepare(args message) (message, bool) {
	// 发来的number小于自身的number 直接返回失败
	if a.promiseNumber >= args.number {
		// 不会发给proposer
		log.Printf("[handle prepare]------- fail. req:%+v me:%+v", args, a)
		return message{}, false
	}
	// 否则, 将自身promiseNumber更新为更大的
	a.promiseNumber = args.number
	msg := message{
		tp:   Promise,
		from: a.id,
		to:   args.from,
		// handle prepared阶段回复给acceptor的number&&value为已经accept的数据
		// 如果此前一旦有accept过, 则该值会永远不变, 且收敛到所有实例都accept该值
		number: a.acceptedNumber,
		value:  a.acceptedValue,
	}
	log.Printf("[handle prepare]********** success! req:%+v me:%+v", args, a)
	return msg, true
}

// Phase 2. (b) If an acceptor receives an accept request for a proposal numbered n,
// it accepts the proposal unless it has already responded to a prepare request
// having a number greater than n.
func (a *acceptor) handleAccept(args message) bool {
	number := args.number
	// handle accept依然比较promiseNumber
	if number >= a.promiseNumber {
		a.acceptedNumber = number
		a.acceptedValue = args.value
		a.promiseNumber = number
		log.Printf("[handle accept] ********* success! req:%+v me:%+v", args, a)
		return true
	}
	log.Printf("[handle accept] ------ fail. req:%+v me:%+v", args, a)
	return false
}
