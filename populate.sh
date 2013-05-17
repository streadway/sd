#!/bin/sh

curl -XPUT -d 'lolcathost:8080' 'http://localhost:8080/zone-[1-9]/product-[1-9]/{prod,test}/{api,admin}/[1-15]:{http,rpc}'
