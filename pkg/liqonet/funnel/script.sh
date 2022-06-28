#!/usr/bin/env bash

sudo ip netns del src1
sudo ip netns del src2
sudo ip netns del gw
sudo ip netns del dst1
sudo ip netns del dst2
sudo ip link del srcbr

set -ex

sleep 1


wg genkey > private-src1
wg genkey > private-src2
wg genkey > private-dst

wg pubkey < private-src1 > public-src1
wg pubkey < private-src2 > public-src2
wg pubkey < private-dst > public-dst

sudo sysctl net.bridge.bridge-nf-call-iptables=0

sudo ip netns add src1
sudo ip netns add src2
sudo ip netns add gw
sudo ip netns add dst1
sudo ip netns add dst2

sudo ip link add srcbr type bridge
sudo ip link add src1-br netns src1 type veth peer name br-src1
sudo ip link add src2-br netns src2 type veth peer name br-src2
sudo ip link add gw-br netns gw type veth peer name br-gw
sudo ip link add gw-dst1 netns gw type veth peer name dst1-gw netns dst1
sudo ip link add gw-dst2 netns gw type veth peer name dst2-gw netns dst2

sudo ip link set srcbr up
sudo ip link set br-src1 up
sudo ip link set br-src2 up
sudo ip link set br-gw up

sudo ip link set br-src1 master srcbr
sudo ip link set br-src2 master srcbr
sudo ip link set br-gw master srcbr

sudo ip netns exec src1 ip link set lo up
sudo ip netns exec src1 ip link set src1-br up
sudo ip netns exec src1 ip address add 169.254.0.1/24 dev src1-br

sudo ip netns exec src2 ip link set lo up
sudo ip netns exec src2 ip link set src2-br up
sudo ip netns exec src2 ip address add 169.254.0.2/24 dev src2-br

sudo ip netns exec gw ip link set lo up
sudo ip netns exec gw ip link set gw-br up
sudo ip netns exec gw ip link set gw-dst1 up
sudo ip netns exec gw ip link set gw-dst2 up
sudo ip netns exec gw ip address add 169.254.0.4/24 dev gw-br
sudo ip netns exec gw ip address add 169.254.4.2/24 dev gw-dst1
sudo ip netns exec gw ip address add 169.254.5.2/24 dev gw-dst2

sudo ip netns exec dst1 ip link set lo up
sudo ip netns exec dst1 ip link set dst1-gw up
sudo ip netns exec dst1 ip address add 169.254.4.1/24 dev dst1-gw
sudo ip netns exec dst1 ip route add 169.254.0.0/24 via 169.254.4.2

sudo ip netns exec dst2 ip link set lo up
sudo ip netns exec dst2 ip link set dst2-gw up
sudo ip netns exec dst2 ip address add 169.254.5.1/24 dev dst2-gw
sudo ip netns exec dst2 ip route add 169.254.0.0/24 via 169.254.5.2

sudo ip netns exec src1 ip link add wg type wireguard
sudo ip netns exec src2 ip link add wg type wireguard
sudo ip netns exec dst1 ip link add wg type wireguard
sudo ip netns exec dst2 ip link add wg type wireguard

sudo ip netns exec src1 ip address add 169.254.8.1/24 dev wg
sudo ip netns exec dst1 ip address add 169.254.8.2/24 dev wg
sudo ip netns exec src2 ip address add 169.254.9.1/24 dev wg
sudo ip netns exec dst2 ip address add 169.254.9.2/24 dev wg

sudo ip netns exec src1 wg set wg private-key ./private-src1
sudo ip netns exec src2 wg set wg private-key ./private-src2
sudo ip netns exec dst1 wg set wg private-key ./private-dst
sudo ip netns exec dst2 wg set wg private-key ./private-dst

sudo ip netns exec src1 wg set wg listen-port 8080
sudo ip netns exec src2 wg set wg listen-port 8080
sudo ip netns exec dst1 wg set wg listen-port 8080
sudo ip netns exec dst2 wg set wg listen-port 8080

sudo ip netns exec src1 ip link set wg up
sudo ip netns exec src2 ip link set wg up
sudo ip netns exec dst1 ip link set wg up
sudo ip netns exec dst2 ip link set wg up

sudo ip netns exec src1 wg set wg peer "$(cat public-dst)" allowed-ips 0.0.0.0/0 endpoint 169.254.0.4:8080
sudo ip netns exec src2 wg set wg peer "$(cat public-dst)" allowed-ips 0.0.0.0/0 endpoint 169.254.0.4:8080

sudo ip netns exec dst1 wg set wg peer "$(cat public-src1)" allowed-ips 0.0.0.0/0 endpoint 169.254.0.1:8080
sudo ip netns exec dst2 wg set wg peer "$(cat public-src2)" allowed-ips 0.0.0.0/0 endpoint 169.254.1.1:8080
