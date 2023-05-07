// Copyright (c) 2018 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

package devicenetwork

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"strings"

	"github.com/lf-edge/eve/pkg/pillar/base"
	"github.com/lf-edge/eve/pkg/pillar/types"
	"github.com/lf-edge/eve/pkg/pillar/zedcloud"
)

// Download a wpad file if so configured
func CheckAndGetNetworkProxy(log *base.LogObject, dns *types.DeviceNetworkStatus,
	ifname string, metrics *zedcloud.AgentMetrics) error {

	portStatus := dns.GetPortByIfName(ifname)
	if portStatus == nil {
		errStr := fmt.Sprintf("Missing port status for interface %s", ifname)
		log.Errorln(errStr)
		return errors.New(errStr)
	}
	proxyConfig := &portStatus.ProxyConfig

	log.Tracef("CheckAndGetNetworkProxy(%s): enable %v, url %s\n",
		ifname, proxyConfig.NetworkProxyEnable,
		proxyConfig.NetworkProxyURL)

	if proxyConfig.Pacfile != "" {
		log.Tracef("CheckAndGetNetworkProxy(%s): already have Pacfile\n",
			ifname)
		return nil
	}
	if !proxyConfig.NetworkProxyEnable {
		log.Tracef("CheckAndGetNetworkProxy(%s): not enabled\n",
			ifname)
		return nil
	}
	if proxyConfig.NetworkProxyURL != "" {
		pac, err := getPacFile(log, proxyConfig.NetworkProxyURL, dns, ifname, metrics)
		if err != nil {
			errStr := fmt.Sprintf("Failed to fetch %s for %s: %s",
				proxyConfig.NetworkProxyURL, ifname, err)
			log.Errorln(errStr)
			return errors.New(errStr)
		}
		proxyConfig.Pacfile = pac
		return nil
	}
	dn := portStatus.DomainName
	if dn == "" {
		errStr := fmt.Sprintf("NetworkProxyEnable for %s but neither a NetworkProxyURL nor a DomainName",
			ifname)
		log.Errorln(errStr)
		return errors.New(errStr)
	}
	log.Functionf("CheckAndGetNetworkProxy(%s): DomainName %s\n",
		ifname, dn)
	// Try http://wpad.%s/wpad.dat", dn where we the leading labels
	// in DomainName until we succeed
	for {
		url := fmt.Sprintf("http://wpad.%s/wpad.dat", dn)
		pac, err := getPacFile(log, url, dns, ifname, metrics)
		if err == nil {
			proxyConfig.Pacfile = pac
			proxyConfig.WpadURL = url
			return nil
		}
		errStr := fmt.Sprintf("Failed to fetch %s for %s: %s",
			url, ifname, err)
		log.Warnln(errStr)
		i := strings.Index(dn, ".")
		if i == -1 {
			log.Functionf("CheckAndGetNetworkProxy(%s): no dots in DomainName %s\n",
				ifname, dn)
			log.Errorln(errStr)
			return errors.New(errStr)
		}
		b := []byte(dn)
		dn = string(b[i+1:])
		// How many dots left? End when we have a TLD i.e., no dots
		// since wpad.com isn't a useful place to look
		count := strings.Count(dn, ".")
		if count == 0 {
			log.Functionf("CheckAndGetNetworkProxy(%s): reached TLD in DomainName %s\n",
				ifname, dn)
			log.Errorln(errStr)
			return errors.New(errStr)
		}
	}
}

func getPacFile(log *base.LogObject, url string, dns *types.DeviceNetworkStatus,
	ifname string, metrics *zedcloud.AgentMetrics) (string, error) {

	zedcloudCtx := zedcloud.NewContext(log, zedcloud.ContextOptions{
		SendTimeout:      15,
		AgentName:        "wpad",
		AgentMetrics:     metrics,
		DevNetworkStatus: dns,
	})
	// Avoid using a proxy to fetch the wpad.dat; 15 second timeout
	const allowProxy = false
	const useOnboard = false
	resp, contents, _, err := zedcloud.SendOnIntf(
		context.Background(), &zedcloudCtx, url, ifname, 0, nil,
		allowProxy, useOnboard, false)
	if err != nil {
		return "", err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		errStr := fmt.Sprintf("%s no content-type\n", url)
		return "", errors.New(errStr)
	}
	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		errStr := fmt.Sprintf("%s ParseMediaType failed %v\n", url, err)
		return "", errors.New(errStr)
	}
	switch mimeType {
	case "application/x-ns-proxy-autoconfig":
		log.Functionf("getPacFile(%s): fetched from URL %s: %s\n",
			ifname, url, string(contents))
		encoded := base64.StdEncoding.EncodeToString(contents)
		return encoded, nil
	default:
		errStr := fmt.Sprintf("Incorrect mime-type %s from %s",
			mimeType, url)
		return "", errors.New(errStr)
	}
}
