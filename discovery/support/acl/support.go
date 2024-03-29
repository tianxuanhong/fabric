/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package acl

import (
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/common"
	common2 "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/pkg/errors"
)

var (
	logger = flogging.MustGetLogger("discovery/acl")
)

// ChannelConfigGetter enables to retrieve the channel config resources
type ChannelConfigGetter interface {
	// GetChannelConfig returns the resources of the channel config
	GetChannelConfig(cid string) channelconfig.Resources
}

// ChannelConfigGetterFunc returns the resources of the channel config
type ChannelConfigGetterFunc func(cid string) channelconfig.Resources

// GetChannelConfig returns the resources of the channel config
func (f ChannelConfigGetterFunc) GetChannelConfig(cid string) channelconfig.Resources {
	return f(cid)
}

// Verifier verifies a signature and a message
type Verifier interface {
	// VerifyByChannel checks that signature is a valid signature of message
	// under a peer's verification key, but also in the context of a specific channel.
	// If the verification succeeded, Verify returns nil meaning no error occurred.
	// If peerIdentity is nil, then the verification fails.
	VerifyByChannel(chainID common.ChainID, peerIdentity api.PeerIdentityType, signature, message []byte) error
}

// Evaluator evaluates signatures.
// It is used to evaluate signatures for the local MSP
type Evaluator interface {
	// Evaluate takes a set of SignedData and evaluates whether this set of signatures satisfies the policy
	Evaluate(signatureSet []*common2.SignedData) error
}

// DiscoverySupport implements support that is used for service discovery
// that is related to access control
type DiscoverySupport struct {
	ChannelConfigGetter
	Verifier
	Evaluator
}

// NewDiscoverySupport creates a new DiscoverySupport
func NewDiscoverySupport(v Verifier, e Evaluator, chanConf ChannelConfigGetter) *DiscoverySupport {
	return &DiscoverySupport{Verifier: v, Evaluator: e, ChannelConfigGetter: chanConf}
}

// Eligible returns whether the given peer is eligible for receiving
// service from the discovery service for a given channel
func (s *DiscoverySupport) EligibleForService(channel string, data common2.SignedData) error {
	if channel == "" {
		return s.Evaluate([]*common2.SignedData{&data})
	}
	return s.VerifyByChannel(common.ChainID(channel), api.PeerIdentityType(data.Identity), data.Signature, data.Data)
}

// ConfigSequence returns the configuration sequence of the given channel
func (s *DiscoverySupport) ConfigSequence(channel string) uint64 {
	// No sequence if the channel is empty
	if channel == "" {
		return 0
	}
	conf := s.GetChannelConfig(channel)
	if conf == nil {
		logger.Panic("Failed obtaining channel config for channel", channel)
	}
	v := conf.ConfigtxValidator()
	if v == nil {
		logger.Panic("ConfigtxValidator for channel", channel, "is nil")
	}
	return v.Sequence()
}

func (s *DiscoverySupport) SatisfiesPrincipal(channel string, rawIdentity []byte, principal *msp.MSPPrincipal) error {
	conf := s.GetChannelConfig(channel)
	if conf == nil {
		return errors.Errorf("channel %s doesn't exist", channel)
	}
	mspMgr := conf.MSPManager()
	if mspMgr == nil {
		return errors.Errorf("could not find MSP manager for channel %s", channel)
	}
	identity, err := mspMgr.DeserializeIdentity(rawIdentity)
	if err != nil {
		return errors.Wrap(err, "failed deserializing identity")
	}
	return identity.SatisfiesPrincipal(principal)
}
