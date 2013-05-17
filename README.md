# Simple Discovery

Status: Under Development

This is an opinionated registry for registering tuples of
zone/product/environment/job/instance/service that map to host:port pairs.

# LICENSE

(waiting until the code settles before a more permissive license)

Copyright Sean Treadway, SoundCloud Ltd. 2013
All Rights Reserved.

# API

## Advertise

Replace a service at a given instance for managed instances.
Prior service on that instance already exists with a different host:port
```
PUT /zone/product/environment/job/instance:service
host:port
```
```
200 OK
del: /zone/product/environment/job/instance:service oldip:oldport
add: /zone/product/environment/job/instance:service ip:port
```

Declare a service at a given instance for managed instances
Prior service at that instance does not exist.

```
PUT /zone/product/environment/job/instance:service
host:port
```
```
201 CREATED
add: /zone/product/environment/job/instance:service ip:port
```

Remove a service
Immediately expire a lease of a known instance.  The entity body must be empty.
```
DELETE /zone/product/environment/job/instance:service
```
```
200 OK
del: /zone/product/environment/job/instance:service ip:port
```
Self managed scheduling
When a cluster scheduler is not involved and service advertisement is stateless, the easiest thing to do is use the leased host:ports as the identifier for advertisement.  This means that the same command can be issued repeatedly to keep a lease alive, without keeping state between lease renewal.
It’s assumed that self-managed lifecycles could stop responding in the case of a hard failure.  The service directory will garbage collect leases that haven’t been renewed.
Self-managed processes can only remove themselves from the service directory if they keep the state of their fully qualified service name so to issue a DELETE like in the case of a managed instance.

Declare a service.
Prior service on that already exists with a different host:port.
A new instance number is found in the first gap or allocated at the end.
Many actual instances could update the same job service, new instances being created when new hosts appear.
```
PUT /zone/product/environment/job:service
host:port
```
```
201 CREATED
Expires: Thu, 01 Dec 2013 16:00:00 GMT

add: /zone/product/environment/job/instance:service ip:port
```

Refresh a service lease.
Prior service exists on any instance with the same host:port.
The instance number of the found host:port pair is found and renewed.
```
PUT /zone/product/environment/job:service
host:port
```
```
200 OK
Expires: Thu, 01 Dec 2013 17:00:00 GMT

add: /zone/product/environment/job/instance:service ip:port
```


Discovery
Assuming service names are injected into runtimes, there are two main use cases - find the host:port pair for a single service instance, or find all host:port pairs for all instances that run that service.

Discover a host:port pair on a specific instance
The instance number of the found host:port pair is found.
```
GET /zone/product/environment/job/instance:service
```
```
200 OK
/zone/product/environment/job/instance:service ip:port
```

Discover a host:port pair on a specific instance
The instance number of the found host:port pair is not advertised, or the lease has expired.
```
GET /zone/product/environment/job/instance:service
```
```
404 NOT FOUND
```

Discover all host:port pairs for a job
The instance number of the found host:port pair has been advertised either by the scheduler or another system.
```
GET /zone/product/environment/job:service
```
```
200 OK
/zone/product/environment/job/0:service ip1:port1
/zone/product/environment/job/1:service ip1:port2
/zone/product/environment/job/3:service ip3:port3
```

## Browsing

Walking the service tree stays within the service namespace.
Browsing is identified by having an incomplete path.
Adding the request header “Accept: text/html” will wrap the results with links `<a href=’{result}’/>`.

Browse all zones
```
GET /
```
```
200 OK
/zone1
/zone2
/zone3
```

Browse for all products in a zone.
```
GET /zone
```
```
200 OK
/zone/product1
/zone/product2
/zone/product3
```

Browse for all environments for a product in a zone.
```
GET /zone/product
```
```
200 OK
/zone/product/environment1
/zone/product/environment2
```

Browse for all jobs for an environment in a product in a zone.
```
GET /zone/product/environment
```
```
200 OK
/zone/product/environment/job1
/zone/product/environment/job2
```

Browse for all services in a job.
Note that this does not return the instances.  To return the instances, use a Query below.
```
GET /zone/product/environment/job
```
```
200 OK
/zone/product/environment/job:service1
/zone/product/environment/job:service2
```
## Querying

Querying is when you wish to discover ip:port pairs when you know some but not all of the names.  Only the instance name can be optionally included.  The rest of the names must either be qualified for an exact match, or an asterisk for any match.

A query is identified by having an asterisk in any path component.

### Query for any service of a specific job of a specific product in any zone or environment.
Without an instance, the results will also not include an instance.
```
GET /*/product/*/job:*
```
```
200 OK
/zone1/product/environment1/job:service1
/zone1/product/environment1/job:service2
/zone1/product/environment2/job:service1
/zone2/product/environment1/job:service1
```

### Query for instances of a specific service of a specific job of a specific product in any zone or environment.
With an instance, the results will contain the instance and only return results for the instances matched.
```
GET /*/product/*/job/*:stats
```
```
200 OK
/zone1/product/environment1/job/0:stats
/zone1/product/environment1/job/1:stats
/zone1/product/environment2/job/0:stats
/zone2/product/environment1/job/0:stats
```

### Query services for a specific instance
```
GET /zone/product/environment/job/0:*
```
```
200 OK
/zone/product/environment/job/0:https
/zone/product/environment/job/0:https-admin
```

### Query which doesn’t match anything.
Return 200 but with an empty body
```
GET /zone/product/environment/*:service
```
```
200 OK
```

## Watches

Updates of changes follows the EventSource working draft.  To get the results of a query and keep the connection alive for updates, pass the Accept: text/event-stream header.  The query will be executed in full on every connection, then further updates will continue to be sent.  The result representation will either be prefixed with an “add: “ or “del: “ event.
Watches are used by client side load balancing, proxy systems or monitoring tools that act on addition and removal of services.

### Query/Browse/Discover with a watch.
```
GET /zone/product/environment/job:service
Accept: text/event-stream
Cache-Control: no-cache
```
```
200 OK
add: /zone/product/environment/job/0:service ip:port1
add: /zone/product/environment/job/1:service ip:port2
del: /zone/product/environment/job/0:service ip:port1
add: /zone/product/environment/job/0:service ip:port3
```

