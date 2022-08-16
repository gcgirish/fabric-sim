// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package basic

import (
	"context"
	"github.com/onosproject/fabric-sim/pkg/utils"
	"github.com/onosproject/fabric-sim/test/client"
	simapi "github.com/onosproject/onos-api/go/onos/fabricsim"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestGNMI loads simulator with access fabric topology, and tests basic gNMI operations
func (s *TestSuite) TestGNMI(t *testing.T) {
	devices := LoadAndValidate(t, "topologies/access_fabric.yaml", 3+6, (3*3*6+3*2)*2, 3*20,
		func(*simapi.Device) int { return 32 }, func(*simapi.Host) int { return 2 })
	defer CleanUp(t)

	device := devices[0]

	conn, err := client.CreateDeviceConnection(device)
	defer func() { _ = conn.Close() }()
	assert.NoError(t, err)
	gnmiClient := gnmi.NewGNMIClient(conn)

	ctx := context.Background()

	// Check basic queries to start
	resp, err := gnmiClient.Get(ctx, &gnmi.GetRequest{
		Path: []*gnmi.Path{utils.ToPath("interfaces/interface[name=...]/state")},
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Notification, 1)
	assert.Len(t, resp.Notification[0].Update, 32*18)

	resp, err = gnmiClient.Get(ctx, &gnmi.GetRequest{
		Path: []*gnmi.Path{utils.ToPath("interfaces/interface[name=...]/state/ifindex")},
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Notification, 1)
	assert.Len(t, resp.Notification[0].Update, 32)

	// Now validate the set... first get value of port enabled
	resp, err = gnmiClient.Get(ctx, &gnmi.GetRequest{
		Path: []*gnmi.Path{utils.ToPath("interfaces/interface[name=3]/config/enabled")},
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Notification, 1)
	assert.Len(t, resp.Notification[0].Update, 1)
	assert.True(t, resp.Notification[0].Update[0].Val.GetBoolVal())

	// Now set the port to disabled
	_, err = gnmiClient.Set(ctx, &gnmi.SetRequest{
		Update: []*gnmi.Update{{
			Path: utils.ToPath("interfaces/interface[name=3]/config/enabled"),
			Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_BoolVal{BoolVal: false}},
		}},
	})
	assert.NoError(t, err)

	// And get it again to see whether it is indeed set to disabled
	resp, err = gnmiClient.Get(ctx, &gnmi.GetRequest{
		Path: []*gnmi.Path{utils.ToPath("interfaces/interface[name=3]/config/enabled")},
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Notification, 1)
	assert.Len(t, resp.Notification[0].Update, 1)
	assert.False(t, resp.Notification[0].Update[0].Val.GetBoolVal())

	// TODO: validate that its state is also disabled
}