//go:build linux

package pi5

import (
	"context"
	"testing"

	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/test"
	rpiutils "raspberry-pi/utils"
)

func TestEmptyBoard(t *testing.T) {
	b := &pinctrlpi5{
		logger: logging.NewTestLogger(t),
	}

	t.Run("test empty sysfs board", func(t *testing.T) {
		_, err := b.GPIOPinByName("10")
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestNewBoard(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	// Create a fake board mapping with two pins for testing.
	// BoardMappings are needed as a parameter passed in to NewBoard but are not used for pin control testing yet.
	testBoardMappings := make(map[string]genericlinux.GPIOBoardMapping, 0)
	conf := &rpiutils.Config{}
	config := resource.Config{
		Name:                "board1",
		ConvertedAttributes: conf,
	}

	// Test Creations of Boards
	newB, err := newBoard(ctx, config, testBoardMappings, logger, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newB, test.ShouldNotBeNil)
	defer newB.Close(ctx)

	// Cast from board.Board to pinctrlpi5 is required to access board's vars
	p5 := newB.(*pinctrlpi5)
	test.That(t, p5.boardPinCtrl.Cfg.ChipSize, test.ShouldEqual, 0x30000)
	testVal := uint64(0x1f000d0000)
	test.That(t, p5.boardPinCtrl.PhysAddr, test.ShouldEqual, testVal)
}
