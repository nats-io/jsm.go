// Copyright 2022 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package natscontext

import (
	"net"
	"net/url"

	"github.com/nats-io/nats.go"
	"golang.org/x/net/proxy"
)

// In context.go we have the regular accessors for the context,
// WithSocksProxy and SocksProxy.
//
// The NATSOptions builder will call SOCKSDialer to get our custom dialer,
// to pass along, if the proxy is set.  The result has to satisfy the
// nats.CustomDialer interface.

// SocksDialer should satisfy the NATS CustomDialer interface
type SocksDialer struct {
	proxy string
}

var _ nats.CustomDialer = SocksDialer{}

func (c *Context) SOCKSDialer() SocksDialer {
	return SocksDialer{proxy: c.SocksProxy()}
}

func (sd SocksDialer) Dial(network, address string) (net.Conn, error) {
	u, err := url.Parse(sd.proxy)
	if err != nil {
		return nil, err
	}
	dialer, err := proxy.FromURL(u, proxy.Direct)
	if err != nil {
		return nil, err
	}
	return dialer.Dial(network, address)
}
