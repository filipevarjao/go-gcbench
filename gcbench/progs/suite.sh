#!/bin/bash

for i in `seq 1 30`; do
	echo " $i activegs " >> activegs_log.txt
	GOMAXPROCS=1 go run activegs.go >> activegs_log.txt
done

for i in `seq 1 30`; do
	echo " $i stackshrink " >> stackshrink_log.txt
	GOMAXPROCS=1 go run stackshrink.go >> stackshrink_log.txt
done

for i in `seq 1 30`; do
	echo " $i smallheap " >> smallheap_log.txt
	GOMAXPROCS=1 go run smallheap.go >> smallheap_log.txt
done

for i in `seq 1 30`; do
	echo " $i rpc " >> rpc_log.txt
	GOMAXPROCS=1 go run rpc.go >> rpc_log.txt
done

for i in `seq 1 30`; do
	echo " $i largeobject " >> largeobject_log.txt
	GOMAXPROCS=1 go run largeobject.go >> largeobject_log.txt
done

for i in `seq 1 30`; do
	echo " $i largeheap " >> largeheap_log.txt
	GOMAXPROCS=1 go run largeheap.go >> largeheap_log.txt
done

for i in `seq 1 30`; do
	echo " $i largebss " >> largebss_log.txt
	GOMAXPROCS=1 go run largebss.go >> largebss_log.txt
done

for i in `seq 1 30`; do
	echo " $i idlegs " >> idlegs_log.txt
	GOMAXPROCS=1 go run idlegs.go >> idlegs_log.txt
done

for i in `seq 1 30`; do
	echo " $i dirtystack " >> dirtystack_log.txt
	GOMAXPROCS=1 go run dirtystack.go >> dirtystack_log.txt
done

echo "\n ThEnd"
