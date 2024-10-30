package rpiutils

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.viam.com/rdk/components/board"
	"go.viam.com/test"
)

func nowNanosecondsTest() uint64 {
	return uint64(time.Now().UnixNano())
}

func TestBasicDigitalInterrupt1(t *testing.T) {
	config := PinConfig{
		Name: "i1",
		Type: "interrupt",
	}

	i, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)

	basicInterrupt := i.(*BasicDigitalInterrupt)

	intVal, err := i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(0))
	test.That(t, Tick(context.Background(), basicInterrupt, true, nowNanosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))
	test.That(t, Tick(context.Background(), basicInterrupt, false, nowNanosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))

	c := make(chan board.Tick)
	AddCallback(basicInterrupt, c)

	timeNanoSec := nowNanosecondsTest()
	go func() { Tick(context.Background(), basicInterrupt, true, timeNanoSec) }()
	time.Sleep(1 * time.Microsecond)
	v := <-c
	test.That(t, v.High, test.ShouldBeTrue)
	test.That(t, v.TimestampNanosec, test.ShouldEqual, timeNanoSec)

	timeNanoSec = nowNanosecondsTest()
	go func() { Tick(context.Background(), basicInterrupt, true, timeNanoSec) }()
	v = <-c
	test.That(t, v.High, test.ShouldBeTrue)
	test.That(t, v.TimestampNanosec, test.ShouldEqual, timeNanoSec)

	RemoveCallback(basicInterrupt, c)

	c = make(chan board.Tick, 2)
	AddCallback(basicInterrupt, c)
	go func() {
		Tick(context.Background(), basicInterrupt, true, uint64(1))
		Tick(context.Background(), basicInterrupt, true, uint64(4))
	}()
	v = <-c
	v1 := <-c
	test.That(t, v.High, test.ShouldBeTrue)
	test.That(t, v1.High, test.ShouldBeTrue)
	test.That(t, v1.TimestampNanosec-v.TimestampNanosec, test.ShouldEqual, uint32(3))
}

func TestRemoveCallbackDigitalInterrupt(t *testing.T) {
	config := PinConfig{
		Name: "d1",
		Type: "interrupt",
	}
	i, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)
	basicInterrupt := i.(*BasicDigitalInterrupt)
	intVal, err := i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(0))
	test.That(t, Tick(context.Background(), basicInterrupt, true, nowNanosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))

	c1 := make(chan board.Tick)
	test.That(t, c1, test.ShouldNotBeNil)
	AddCallback(basicInterrupt, c1)
	var wg sync.WaitGroup
	wg.Add(1)
	ret := false

	go func() {
		defer wg.Done()
		select {
		case <-context.Background().Done():
			return
		default:
		}
		select {
		case <-context.Background().Done():
			return
		case tick := <-c1:
			ret = tick.High
		}
	}()
	test.That(t, Tick(context.Background(), basicInterrupt, true, nowNanosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(2))
	wg.Wait()
	c2 := make(chan board.Tick)
	test.That(t, c2, test.ShouldNotBeNil)
	AddCallback(basicInterrupt, c2)
	test.That(t, ret, test.ShouldBeTrue)

	RemoveCallback(basicInterrupt, c1)
	RemoveCallback(basicInterrupt, c1)

	ret2 := false
	result := make(chan bool, 1)
	go func() {
		defer wg.Done()
		select {
		case <-context.Background().Done():
			return
		default:
		}
		select {
		case <-context.Background().Done():
			return
		case tick := <-c2:
			ret2 = tick.High
		}
	}()
	wg.Add(1)
	go func() {
		err := Tick(context.Background(), basicInterrupt, true, nowNanosecondsTest())
		if err != nil {
			result <- true
		}
		result <- true
	}()
	select {
	case <-time.After(1 * time.Second):
		ret = false
	case ret = <-result:
	}
	wg.Wait()
	test.That(t, ret, test.ShouldBeTrue)
	test.That(t, ret2, test.ShouldBeTrue)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(3))
}
