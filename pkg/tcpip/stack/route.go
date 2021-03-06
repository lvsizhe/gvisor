// Copyright 2016 The Netstack Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stack

import (
	"gvisor.googlesource.com/gvisor/pkg/sleep"
	"gvisor.googlesource.com/gvisor/pkg/tcpip"
	"gvisor.googlesource.com/gvisor/pkg/tcpip/buffer"
	"gvisor.googlesource.com/gvisor/pkg/tcpip/header"
)

// Route represents a route through the networking stack to a given destination.
type Route struct {
	// RemoteAddress is the final destination of the route.
	RemoteAddress tcpip.Address

	// RemoteLinkAddress is the link-layer (MAC) address of the
	// final destination of the route.
	RemoteLinkAddress tcpip.LinkAddress

	// LocalAddress is the local address where the route starts.
	LocalAddress tcpip.Address

	// LocalLinkAddress is the link-layer (MAC) address of the
	// where the route starts.
	LocalLinkAddress tcpip.LinkAddress

	// NextHop is the next node in the path to the destination.
	NextHop tcpip.Address

	// NetProto is the network-layer protocol.
	NetProto tcpip.NetworkProtocolNumber

	// ref a reference to the network endpoint through which the route
	// starts.
	ref *referencedNetworkEndpoint
}

// makeRoute initializes a new route. It takes ownership of the provided
// reference to a network endpoint.
func makeRoute(netProto tcpip.NetworkProtocolNumber, localAddr, remoteAddr tcpip.Address, ref *referencedNetworkEndpoint) Route {
	return Route{
		NetProto:      netProto,
		LocalAddress:  localAddr,
		RemoteAddress: remoteAddr,
		ref:           ref,
	}
}

// NICID returns the id of the NIC from which this route originates.
func (r *Route) NICID() tcpip.NICID {
	return r.ref.ep.NICID()
}

// MaxHeaderLength forwards the call to the network endpoint's implementation.
func (r *Route) MaxHeaderLength() uint16 {
	return r.ref.ep.MaxHeaderLength()
}

// PseudoHeaderChecksum forwards the call to the network endpoint's
// implementation.
func (r *Route) PseudoHeaderChecksum(protocol tcpip.TransportProtocolNumber) uint16 {
	return header.PseudoHeaderChecksum(protocol, r.LocalAddress, r.RemoteAddress)
}

// Capabilities returns the link-layer capabilities of the route.
func (r *Route) Capabilities() LinkEndpointCapabilities {
	return r.ref.ep.Capabilities()
}

// Resolve attempts to resolve the link address if necessary. Returns ErrWouldBlock in
// case address resolution requires blocking, e.g. wait for ARP reply. Waker is
// notified when address resolution is complete (success or not).
func (r *Route) Resolve(waker *sleep.Waker) *tcpip.Error {
	if !r.IsResolutionRequired() {
		// Nothing to do if there is no cache (which does the resolution on cache miss) or
		// link address is already known.
		return nil
	}

	nextAddr := r.NextHop
	if nextAddr == "" {
		nextAddr = r.RemoteAddress
	}
	linkAddr, err := r.ref.linkCache.GetLinkAddress(r.ref.nic.ID(), nextAddr, r.LocalAddress, r.NetProto, waker)
	if err != nil {
		return err
	}
	r.RemoteLinkAddress = linkAddr
	return nil
}

// RemoveWaker removes a waker that has been added in Resolve().
func (r *Route) RemoveWaker(waker *sleep.Waker) {
	nextAddr := r.NextHop
	if nextAddr == "" {
		nextAddr = r.RemoteAddress
	}
	r.ref.linkCache.RemoveWaker(r.ref.nic.ID(), nextAddr, waker)
}

// IsResolutionRequired returns true if Resolve() must be called to resolve
// the link address before the this route can be written to.
func (r *Route) IsResolutionRequired() bool {
	return r.ref.linkCache != nil && r.RemoteLinkAddress == ""
}

// WritePacket writes the packet through the given route.
func (r *Route) WritePacket(hdr *buffer.Prependable, payload buffer.View, protocol tcpip.TransportProtocolNumber) *tcpip.Error {
	return r.ref.ep.WritePacket(r, hdr, payload, protocol)
}

// MTU returns the MTU of the underlying network endpoint.
func (r *Route) MTU() uint32 {
	return r.ref.ep.MTU()
}

// Release frees all resources associated with the route.
func (r *Route) Release() {
	if r.ref != nil {
		r.ref.decRef()
		r.ref = nil
	}
}

// Clone Clone a route such that the original one can be released and the new
// one will remain valid.
func (r *Route) Clone() Route {
	r.ref.incRef()
	return *r
}
