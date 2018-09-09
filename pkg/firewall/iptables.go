// +build linux

// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This is a "meta-plugin". It reads in its own netconf, it does not create
// any network interface but just changes the network sysctl.

package firewall

import (
	"fmt"
	"net"

	"github.com/coreos/go-iptables/iptables"
)

func getPrivChainRules(ip string) [][]string {
	var rules [][]string
	rules = append(rules, []string{"-d", ip, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"})
	rules = append(rules, []string{"-s", ip, "-j", "ACCEPT"})
	return rules
}

func ensureChain(ipt *iptables.IPTables, table, chain string) error {
	chains, err := ipt.ListChains(table)
	if err != nil {
		return fmt.Errorf("failed to list iptables chains: %v", err)
	}
	for _, ch := range chains {
		if ch == chain {
			return nil
		}
	}

	return ipt.NewChain(table, chain)
}

func generateFilterRule(privChainName string) []string {
	return []string{"-m", "comment", "--comment", "CNI firewall plugin rules", "-j", privChainName}
}

func generateAdminRule(adminChainName string) []string {
	return []string{"-m", "comment", "--comment", "CNI firewall plugin admin overrides", "-j", adminChainName}
}

func cleanupRules(ipt *iptables.IPTables, privChainName string, rules [][]string) {
	for _, rule := range rules {
		ipt.Delete("filter", privChainName, rule...)
	}
}

func ensureFirstChainRule(ipt *iptables.IPTables, chain string, rule []string) error {
	exists, err := ipt.Exists("filter", chain, rule...)
	if !exists && err == nil {
		err = ipt.Insert("filter", chain, 1, rule...)
	}
	return err
}

func (ib *iptablesBackend) setupChains(ipt *iptables.IPTables) error {
	privRule := generateFilterRule(ib.privChainName)
	adminRule := generateFilterRule(ib.adminChainName)

	// Ensure our private chains exist
	if err := ensureChain(ipt, "filter", ib.privChainName); err != nil {
		return err
	}
	if err := ensureChain(ipt, "filter", ib.adminChainName); err != nil {
		return err
	}

	// Ensure our filter rule exists in the forward chain
	if err := ensureFirstChainRule(ipt, "FORWARD", privRule); err != nil {
		return err
	}

	// Ensure our admin override chain rule exists in our private chain
	if err := ensureFirstChainRule(ipt, ib.privChainName, adminRule); err != nil {
		return err
	}

	return nil
}

func protoForIP(ip net.IPNet) iptables.Protocol {
	if ip.IP.To4() != nil {
		return iptables.ProtocolIPv4
	}
	return iptables.ProtocolIPv6
}

func (ib *iptablesBackend) addRules(conf *FirewallNetConf, ipt *iptables.IPTables, proto iptables.Protocol) error {
	rules := make([][]string, 0)
	for _, ip := range conf.PrevResult.IPs {
		if protoForIP(ip.Address) == proto {
			rules = append(rules, getPrivChainRules(ipString(ip.Address))...)
		}
	}

	if len(rules) > 0 {
		if err := ib.setupChains(ipt); err != nil {
			return err
		}

		// Clean up on any errors
		var err error
		defer func() {
			if err != nil {
				cleanupRules(ipt, ib.privChainName, rules)
			}
		}()

		for _, rule := range rules {
			err = ipt.AppendUnique("filter", ib.privChainName, rule...)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ib *iptablesBackend) delRules(conf *FirewallNetConf, ipt *iptables.IPTables, proto iptables.Protocol) error {
	rules := make([][]string, 0)
	for _, ip := range conf.PrevResult.IPs {
		if protoForIP(ip.Address) == proto {
			rules = append(rules, getPrivChainRules(ipString(ip.Address))...)
		}
	}

	if len(rules) > 0 {
		cleanupRules(ipt, ib.privChainName, rules)
	}

	return nil
}

func findProtos(conf *FirewallNetConf) []iptables.Protocol {
	protos := []iptables.Protocol{iptables.ProtocolIPv4, iptables.ProtocolIPv6}
	if conf.PrevResult != nil {
		// If PrevResult is given, scan all IP addresses to figure out
		// which IP versions to use
		protos = []iptables.Protocol{}
		for _, addr := range conf.PrevResult.IPs {
			if addr.Address.IP.To4() != nil {
				protos = append(protos, iptables.ProtocolIPv4)
			} else {
				protos = append(protos, iptables.ProtocolIPv6)
			}
		}
	}
	return protos
}

type iptablesBackend struct {
	protos         map[iptables.Protocol]*iptables.IPTables
	privChainName  string
	adminChainName string
	ifName         string
}

// iptablesBackend implements the FirewallBackend interface
var _ FirewallBackend = &iptablesBackend{}

func newIptablesBackend() (FirewallBackend, error) {
	adminChainName := "CNI-ADMIN"

	backend := &iptablesBackend{
		privChainName:  "CNI-FORWARD",
		adminChainName: adminChainName,
		protos:         make(map[iptables.Protocol]*iptables.IPTables),
	}

	for _, proto := range []iptables.Protocol{iptables.ProtocolIPv4, iptables.ProtocolIPv6} {
		ipt, err := iptables.NewWithProtocol(proto)
		if err != nil {
			return nil, fmt.Errorf("could not initialize iptables protocol %v: %v", proto, err)
		}
		backend.protos[proto] = ipt
	}

	return backend, nil
}

func (ib *iptablesBackend) Add(conf *FirewallNetConf) error {
	for proto, ipt := range ib.protos {
		if err := ib.addRules(conf, ipt, proto); err != nil {
			return err
		}
	}
	return nil
}

func (ib *iptablesBackend) Del(conf *FirewallNetConf) error {
	for proto, ipt := range ib.protos {
		ib.delRules(conf, ipt, proto)
	}
	return nil
}
