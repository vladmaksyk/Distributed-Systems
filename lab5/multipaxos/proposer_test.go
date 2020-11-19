package multipaxos

import (
	"reflect"
	"testing"
)

func TestHandlePromise(t *testing.T) {
	testHandlePromise(proposerTestsThreeNodes, t)
	testHandlePromise(proposerTestsFiveNodes, t)
}

func testHandlePromise(tests []proposerTest, t *testing.T) {
	for i, test := range tests {
		for j, action := range test.actions {
			gotAccs, gotOutput := test.proposer.handlePromise(action.promise)
			switch {
			case !action.wantOutput && gotOutput:
				t.Errorf("\nHandlePromise\ntest nr:%d\naction nr: %d\ndescription: %s\nwant no output\ngot %v",
					i+1, j+1, action.desc, gotAccs)
			case action.wantOutput && !gotOutput:
				t.Errorf("\nHandlePromise\ntest nr:%d\naction nr: %d\ndescription: %s\nwant %v\ngot no output",
					i+1, j+1, action.desc, action.wantAccs)
			case action.wantOutput && gotOutput:
				if !reflect.DeepEqual(gotAccs, action.wantAccs) {
					t.Errorf("\nHandlePromise\ntest nr:%d\naction nr: %d\ndescription: %s\nwant:\t%v\ngot:\t%v",
						i+1, j+1, action.desc, action.wantAccs, gotAccs)
				}
			}
		}
	}
}

type proposerTest struct {
	proposer *Proposer
	actions  []paction
}

type paction struct {
	promise    Promise
	desc       string
	wantOutput bool
	wantAccs   []Accept
}

var proposerTestsThreeNodes = []proposerTest{
	{
		NewProposer(2, 3, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  2,},
				"single promise from 1 with correct round, no quorum -> no output",
				false,
				nil,
			},
		},
	},
	{
		NewProposer(2, 3, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  2, },
				"promise from 1 with correct round, no quorum -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 0, Rnd:  2,},
				"promise from 0, with correct round, quorum -> output and emtpy accept slice ",
				true,
				[]Accept{},
			},
		},
	},
	{
		NewProposer(2, 3, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  42, },
				"promise from 1 with different round (42) than proposer's (2), ignore -> no output",
				false,
				nil,
			},
		},
	},
	{
		NewProposer(2, 3, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  1, },
				"promise from 1 with different round (1) than proposer's (2), ignore -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 1, Rnd:  6, },
				"promise from 1 with different round (6) than proposer's (2), ignore -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 0, Rnd:  4, },
				"promise from 0 with different round (4) than proposer's (2), ignore -> no output",
				false,
				nil,
			},
		},
	},
	{
		NewProposer(2, 3, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  2,},
				"promise from 1 with correct round (2), no quorum -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 1, Rnd:  2,},
				"again promise from 1 with correct round (2), ignore, no quorum -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 1, Rnd:  2,},
				"again promise from 1 with correct round (2), ignore, no quorum -> no output",
				false,
				nil,
			},
		},
	},
	{
		NewProposer(2, 3, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  6,},
				"promise from 1 with different round (6) than proposer's (2), ignore -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 1, Rnd:  6,},
				"promise from 0 with different round (6) than proposer's (2), quorum for round 6, not but our round, ignore -> no output",
				false,
				nil,
			},
		},
	},
	{
		NewProposer(2, 3, 1, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  2, },
				"Promise from 1 for correct round. No PromiseSlot reported. No quorum.",
				false,
				nil,
			},
			{
				Promise{ To:    2, From:  0, Rnd:   2, Slots: []PromiseSlot{{ID: 2, Vrnd: 1, Vval: testingValueOne}},},
				"Promise from 0 for correct round . Single PromiseSlot for slot 2. Quorum. Output Accept for slot 2 with value reported by 0.",
				true,
				[]Accept{{From: 2, Slot: 2, Rnd: 2, Val: testingValueOne}},
			},
		},
	},
	{
		NewProposer(2, 3, 1, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:    2, From:  1, Rnd:   2, Slots: []PromiseSlot{{ID: 2, Vrnd: 0, Vval: testingValueOne}},},
				"Promise from 1 for correct round. Single PromiseSlot reported for slot 2 with vrnd 0. No quorum.",
				false,
				nil,
			},
			{
				Promise{ To:    2, From:  0, Rnd:   2, Slots: []PromiseSlot{{ID: 2, Vrnd: 1, Vval: testingValueTwo}},},
				"Promise from 0 for correct round.Single PromiseSlot reported for slot 2 with vrnd 1. Quorum. Output Accept for slot 2 with value reported by 0 due to higher vrnd. ",
				true,
				[]Accept{{From: 2, Slot: 2, Rnd: 2, Val: testingValueTwo}},
			},
		},
	},
	{
		NewProposer(2, 3, 1, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  2, Slots: []PromiseSlot{{ID: 1, Vrnd: 0, Vval: testingValueTwo},{ID: 2, Vrnd: 0, Vval: testingValueOne},},},
				"Promise from 1 for correct round. Two PromiseSlot reported for slot 1 and 2 with vrnd 0. Note that Slot 1 is lower than Proposer adu. No quorum.",
				false,
				nil,
			},
			{
				Promise{ To:    2, From:  0, Rnd:   2, Slots: []PromiseSlot{{ID: 2, Vrnd: 1, Vval: testingValueTwo}},},
				"Promise from 0 for correct round. Single PromiseSlot for slot 2 with vrnd 1. Quorum. Output Accept for slot 2 with value reported by 0 due to higher vrnd. Slot 1 reported by node 1 should be ignored since its lower than Proposer adu. Should not happen but we check it for completeness.",
				true,
				[]Accept{{From: 2, Slot: 2, Rnd: 2, Val: testingValueTwo}},
			},
		},
	},
	{
		NewProposer(2, 3, 1, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{To:   2, From: 1, Rnd:  2, Slots: []PromiseSlot{ {ID: 2, Vrnd: 1, Vval: testingValueOne}, {ID: 4, Vrnd: 1, Vval: testingValueThree},},},
				"Promise from 1 for correct round. Two PromiseSlot reported for slot 2 and 4 with vrnd 1. Note the gap between 2 and 4. No quorum",
				false,
				nil,
			},
			{
				Promise{To:    2, From:  0, Rnd:   2, Slots: []PromiseSlot{{ID: 2, Vrnd: 1, Vval: testingValueOne}},},
				"Promise from 0 for correct round. Single PromiseSlot reported for slot 2 with vrnd 1. Quorum. Output Accept for slot 2, 3 and 4. Slot 1 with value reported by node 0 and 1(equal vrnd). Slot 4 with value reported by node 1. Slot 3 is a gap and needs to be filled with a noop.",
				true,
				[]Accept{
					{From: 2, Slot: 2, Rnd: 2, Val: testingValueOne},
					{From: 2, Slot: 3, Rnd: 2, Val: Value{Noop: true}},
					{From: 2, Slot: 4, Rnd: 2, Val: testingValueThree},
				},
			},
		},
	},
	{
		NewProposer(2, 3, 1, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  2, Slots: []PromiseSlot{{ID: 2, Vrnd: 0, Vval: testingValueOne},
																							{ID: 4, Vrnd: 0, Vval: testingValueThree},
																							{ID: 5, Vrnd: 1, Vval: testingValueTwo},},},
				"Promise from 1 for correct round. Three PromiseSlot reported for slot 2,4 and 5. Note the gap between 2 and 4. No quorum.",
				false,
				nil,
			},
			{
				Promise{To:   2, From: 1, Rnd:  2, Slots: []PromiseSlot{ {ID: 2, Vrnd: 0, Vval: testingValueOne},
																							{ID: 4, Vrnd: 0, Vval: testingValueThree},
																							{ID: 5, Vrnd: 1, Vval: testingValueTwo},},},
				"Duplicate prommise from 1, equal as message above. Since we already have a Promise from 1 for round 2 we ignore it. No quorum.",
				false,
				nil,
			},
			{
				Promise{To:   2, From: 0, Rnd:  2, Slots: []PromiseSlot{ {ID: 2, Vrnd: 0, Vval: testingValueOne},
																							{ID: 4, Vrnd: 1, Vval: testingValueTwo}, },},
				"Promise from 0 for correct round. Two PromiseSlot reported for slot 2 and 4. Quorum. Output Accept for slot 2,3,4 and 5. Slot 1 with value reported by node 0 and 1 (equal vrnd). Slot 3 is a gap and therefoor a noop. Slot 4 with value reported by node 0 due to higher vrnd. Slot 5 with value reported by node 1(only node 1 reported a value for this slot) ",
				true,
				[]Accept{{From: 2, Slot: 2, Rnd: 2, Val: testingValueOne},
							{From: 2, Slot: 3, Rnd: 2, Val: Value{Noop: true}},
							{From: 2, Slot: 4, Rnd: 2, Val: testingValueTwo},
							{From: 2, Slot: 5, Rnd: 2, Val: testingValueTwo},},
			},
		},
	},
	{
		NewProposer(2, 3, 1, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  2, Slots: []PromiseSlot{ {ID: 2, Vrnd: 0, Vval: testingValueOne},
																							 {ID: 4, Vrnd: 1, Vval: testingValueThree},
																							 {ID: 5, Vrnd: 1, Vval: testingValueTwo},},},
				"variation of scenario 5 - study test code for details",
				false,
				nil,
			},
			{
				Promise{To:   2, From: 1, Rnd:  2, Slots: []PromiseSlot{ {ID: 2, Vrnd: 0, Vval: testingValueOne},
																							{ID: 4, Vrnd: 1, Vval: testingValueThree},
																							{ID: 5, Vrnd: 1, Vval: testingValueTwo},},},
				"variation of scenario 5 - study test code for details",
				false,
				nil,
			},
			{
				Promise{To:   2, From: 0, Rnd:  2, Slots: []PromiseSlot{ {ID: 2, Vrnd: 0, Vval: testingValueOne},
																							{ID: 4, Vrnd: 0, Vval: testingValueTwo}, },},
				"variation of scenario 5 - study test code for details",
				true,
				[]Accept{
					{From: 2, Slot: 2, Rnd: 2, Val: testingValueOne},
					{From: 2, Slot: 3, Rnd: 2, Val: Value{Noop: true}},
					{From: 2, Slot: 4, Rnd: 2, Val: testingValueThree},   //saves the VRnd 0 but has to save 1
					{From: 2, Slot: 5, Rnd: 2, Val: testingValueTwo},
				},
			},
		},
	},
}

var proposerTestsFiveNodes = []proposerTest{
	{
		NewProposer(2, 5, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  2, },
				"single promise from 1 with correct round, n=5, no quorum -> no output",
				false,
				nil,
			},
		},
	},
	{
		NewProposer(2, 5, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  2, },
				"promise from 1 with correct round, n=5, no quorum -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 3, Rnd:  2,},
				"promise from 3 with correct round, n=5, no quorum -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 4, Rnd:  2,},
				"promise from 4, with correct round, n=5, quorum -> output and emtpy accept slice ",
				true,
				[]Accept{},
			},
		},
	},
	{
		NewProposer(2, 5, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{To:   2,From: 1,Rnd:  42,},
				"promise from 1 with different round (42) than proposer's (2), n=5, ignore -> no output",
				false,
				nil,
			},
		},
	},
	{
		NewProposer(2, 5, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{To:   2, From: 1, Rnd:  1,},
				"promise from 1 with different round (1) than proposer's (2), n=5, ignore -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 2, Rnd:  6,},
				"promise from 2 with different round (6) than proposer's (2), n=5, ignore -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 4, Rnd:  4,},
				"promise from 4 with different round (4) than proposer's (2), n=5, ignore -> no output",
				false,
				nil,
			},
		},
	},
	{
		NewProposer(2, 5, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{To:   2, From: 1, Rnd:  2,},
				"promise from 1 with correct round (2), n=5, no quorum -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 1, Rnd:  2,},
				"again promise from 1 with correct round (2), n=5, ignore, no quorum -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 1, Rnd:  2,},
				"again promise from 1 with correct round (2), n=5, ignore, no quorum -> no output",
				false,
				nil,
			},
		},
	},
	{
		NewProposer(2, 5, 0, &mockLD{}, nil, nil),
		[]paction{
			{
				Promise{ To:   2, From: 1, Rnd:  6,},
				"promise from 4 with different round (6) than proposer's (2), n=5, ignore -> no output",
				false,
				nil,
			},
			{
				Promise{To:   2, From: 3, Rnd:  6,},
				"promise from 3 with different round (6) than proposer's (2), n=5, ignore -> no output",
				false,
				nil,
			},
			{
				Promise{ To:   2, From: 4, Rnd:  6,},
				"promise from 4 with different round (6) than proposer's (2), n=5, quorum for round 6, not but our round, ignore -> no output",
				false,
				nil,
			},
		},
	},
}

type mockLD struct{}

func (l *mockLD) Leader() int {
	return -1
}

func (l *mockLD) Subscribe() <-chan int {
	return make(chan int)
}
