# Automate DDNS using Kubernetes

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
- [Design Details](#design-details)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)

## Summary

Centrally manage multiple Dynamic DNS (DDNS) records in DuckDNS.org using Kubernetes.

## Motivation

The motivation behind this project is to manage a home lab using free dynamic (DDNS) services.
Since the free DDNS services are limited to either single domain name or can only create subdomains, it would be
challenging to manage multiple domain names or subdomains manually.

Since Kubernetes is hosting the applications that make up my home network, it is easier if I can manage the
DNS records using Kubernetes as well.

Dynamic DNS (DDNS) is a service that automatically updates the DNS records of a domain name when the IP address of
the host changes.
This is particularly useful for home networks where the public IP address may change frequently, such as with
residential ISPs that do not provide static IP addresses.

### Goals

- Automate the process of updating DNS records using Kubernetes.
- Provides a simple and easy-to-use interface for managing DNS records.
- Make it easy to tear down the setup and clean up resources.

## Design Details

Design is pretty basic.

1. A Kubernetes CR (Custom Resource) is created to request managing a DDNS record.
2. A Kubernetes is a controller watching for DNS Record CR and creates a CronJob to keep the DNS record updated.
3. When the DNS Record is updated, the controller updates the script that runs in the CronJob to update the DNS record.
4. The CronJob runs periodically to update the DNS record with the current public IP address of the host.
5. To remove the DNS record, delete the DNS Record CR, and the controller removes the CronJob and cleans up resources.

## Drawbacks

1. Complexity: Adding another layer of abstraction to manage DNS records can increase the complexity of the system.
   - Pros: Centralized management, easier to maintain.
   - Cons: More components to manage, potential for more points of failure.

## Alternatives

1. Cron jobs:Use a cron job to periodically update the DNS records using a script.
   - Pros: Simple to implement, no additional dependencies.
   - Cons: Requires manual setup and maintenance, not as flexible.
