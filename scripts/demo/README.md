# Demo

This demo will guide you through setting up a static website served over HTTPS using DuckDNS and Kubernetes.

## Prepare the environment

### Create a kind cluster

```shell
kind create cluster --config ./scripts/demo/kind.yaml
```

### Install nginx-ingress

```shell
kubectl apply -f https://kind.sigs.k8s.io/examples/ingress/deploy-ingress-nginx.yaml
```

### Install static website

Static website is used to serve the DuckDNS domain name.

#### Prepare docker image

```shell
docker build -t static:latest -f ./scripts/demo/Dockerfile .
```

##### Load image into the kind cluster

```shell
kind load --name duckdns docker-image static:latest
```

#### Deploy static website

```shell
kubectl apply -f ./scripts/demo/website.yaml
```

#### Create ingress for the static website

```shell
kubectl apply -f ./scripts/demo/ingress.yaml
````

**Note:** The IP address of the kind cluster host machine will be used to access the static website.

```shell
ifconfig | grep -A5 en0 | grep inet
	inet6 fe80::1832:ba1e:93d3:7c79%en0 prefixlen 64 secured scopeid 0xe
	inet 192.168.50.87 netmask 0xffffff00 broadcast 192.168.50.255
```

## Access the static website from public IP

Login to your router and forward port 80 to the IP address of the kind cluster host machine.
You can access the static website using the public IP address of your router.
Each router has a different way to set up port forwarding, so refer to your router's documentation for instructions.

To find your public IP address, run the following command:

```shell
curl ifconfig.me
```

Then visit `http://<your-public-ip>` in your web browser.

## Access the static website from DuckDNS domain

Visit [DuckDNS.ORG](https://www.duckdns.org/) and create an account.
Then create a new domain name, for example `my-domain.duckdns.org`.
Next, update the `ingress.yaml` and add host to the ingress rule and apply the changes:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: website-ingress
spec:
    rules:
    - host: "my-domain.duckdns.org"
      # other configurations
```

```shell
kubectl apply -f ./scripts/demo/ingress.yaml
```

## Secure the static website with TLS

### Install cert-manager

```shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.3/cert-manager.yaml
```

#### Add cert-manager issuers

```shell
kubectl apply -f ./scripts/demo/cert_issuers.yaml
```

### Update ingress to use TLS

Update the `ingress.yaml` to use TLS and add the secret name for the certificate:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: website-ingress
  labels:
    name: website
  annotations:
    cert-manager.io/cluster-issuer: lets-encrypt-staging
    nginx.ingress.kubernetes.io/limit-connections: "1000"
    nginx.ingress.kubernetes.io/limit-rps: "1000"
    nginx.ingress.kubernetes.io/limit-rpm: "60000"
    nginx.ingress.kubernetes.io/proxy-body-size: 5m
spec:
  ingressClassName: nginx
  rules:
    - host: "domain.duckdns.org"
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: website
                port:
                  number: 80
  tls:
    - hosts:
        - "domain.duckdns.org"
      secretName: website-ingress-cert
````

Then apply the changes:

```shell
kubectl apply -f ./scripts/demo/ingress.yaml
```

### Verify the TLS certificate

You can verify the TLS certificate by visiting `https://domain.duckdns.org` in your web browser.
You should see a valid TLS certificate issued by Let's Encrypt.
If you see a warning about the certificate being invalid, it may take a few minutes for the certificate to be issued and propagated. You can check the status of the certificate by running:

```shell
kubectl describe certificate website-ingress-cert
```

### Add 443 port-forwarding

To access the static website over HTTPS, you need to forward port 443 to the IP address of the kind cluster host machine.
You can do this by adding a port forwarding rule in your router's settings, similar to how you did for port 80.
To find your public IP address, run the following command:

### Let's encrypt staging

Since the cert is issued by Let's Encrypt staging, it is not valid for production use, 
and you will see a warning in your web browser.

To use a production certificate, update the ingress to use the production issuer by changing the annotation
`cert-manager.io/cluster-issuer: lets-encrypt-staging` to `cert-manager.io/cluster-issuer: lets-encrypt-production`
and apply the changes:

### Remove port 80 port-forwarding

Since the static website is now accessible over HTTPS, you can remove the port 80 port forwarding rule
from your router's settings so that the website is only accessible over HTTPS.

Done. You can now access your static website securely over HTTPS using your DuckDNS domain name.

## Automatically update the DuckDNS domain

To automatically update the DuckDNS domain name with your public IP address, deploy k8s-duckdns:

```shell
make docker-build
```

Load the image into the kind cluster:

```shell
kind load --name duckdns docker-image controller:latest
```

Finally, deploy the controller:

```shell
make deploy
```

### Set up the DNS record

Run the `generate` command to create a DNS record and a DNS record secret for the dns record.

```shell
./scripts/generate dns-record --domains domain.duckdns.org
```

Run `./scripts/generate -h` to see all the available options.

Copy the DuckDNS token from the DuckDNS website.

Then create a secret with the DuckDNS token:

```shel
./scripts/generate secret --token 563ef6f6-4028-11f0-9531-bf58ce28667e
```

Run `./scripts/generate secret -h` to see all the available options.

Finally, apply the generated DNS record and secret:

```shell
kubectl apply -f ./dns-record.yaml
```

```shell
kubectl apply -f ./dns-record-secret.yaml
```

### Verify the CronJob is running

```shell
kubectl get cronjobs
NAME                     SCHEDULE      TIMEZONE   SUSPEND   ACTIVE   LAST SCHEDULE   AGE
duckdns-record-cronjob   */5 * * * *   <none>     False     0        <none>          28s
```

## Voila!

You've got a fully functional static website served over HTTPS using DuckDNS and Kubernetes.

## Clean up

To clean up the resources created during the demo, delete the kind cluster.

```shell
kind delete cluster --name duckdns
```

Then delete the domain name from DuckDNS.
