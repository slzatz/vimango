# Setting Up Secure PostgreSQL with SSL/TLS

This guide documents how to enable encrypted connections between vimango and a remote PostgreSQL server using self-signed certificates.

## Overview

- **Server**: PostgreSQL on remote host
- **Client**: vimango on laptop
- **Authentication**: Password + SSL certificate verification
- **Encryption**: TLS for all data in transit

## Part 1: Generate Certificates (on the server)

> **Note**: The commands below use the legacy Common Name (CN) field for the server IP address.
> This approach supports `verify-ca` mode only. For `verify-full` mode, see
> [Upgrading to verify-full Mode](#upgrading-to-verify-full-mode) below.

```bash
# Create a directory for certificate generation
mkdir -p ~/pg_certs && cd ~/pg_certs

# 1. Create your own Certificate Authority (valid 10 years)
openssl req -new -x509 -days 3650 -nodes \
  -out ca.crt -keyout ca.key \
  -subj "/CN=My Personal CA"

# 2. Create server key and certificate signing request
#    Replace the IP with your server's IP address
openssl req -new -nodes \
  -out server.csr -keyout server.key \
  -subj "/CN=96.126.108.62"

# 3. Sign the server certificate with your CA (valid 2 years)
openssl x509 -req -in server.csr -days 730 \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt

# 4. Verify the certificates were created
ls -la ~/pg_certs/
# Should see: ca.crt, ca.key, server.crt, server.key, server.csr
```

## Part 2: Install Certificates for PostgreSQL (on the server)

PostgreSQL runs as the `postgres` system user, which cannot access files in your home directory. Move certificates to a system location:

```bash
# Create SSL directory for PostgreSQL
sudo mkdir -p /etc/postgresql/ssl

# Copy certificates
sudo cp ~/pg_certs/server.crt /etc/postgresql/ssl/
sudo cp ~/pg_certs/server.key /etc/postgresql/ssl/
sudo cp ~/pg_certs/ca.crt /etc/postgresql/ssl/

# Set ownership to postgres user
sudo chown postgres:postgres /etc/postgresql/ssl/*

# Set permissions (private key must be 600)
sudo chmod 600 /etc/postgresql/ssl/server.key
sudo chmod 644 /etc/postgresql/ssl/server.crt /etc/postgresql/ssl/ca.crt
```

## Part 3: Configure PostgreSQL (on the server)

Edit `postgresql.conf` (typically at `/etc/postgresql/<version>/main/postgresql.conf`):

```
ssl = on
ssl_cert_file = '/etc/postgresql/ssl/server.crt'
ssl_key_file = '/etc/postgresql/ssl/server.key'
ssl_ca_file = '/etc/postgresql/ssl/ca.crt'
```

Optionally, update `pg_hba.conf` to require SSL for remote connections:

```
# Require SSL for remote connections (hostssl instead of host)
hostssl    vimango    slzatz    0.0.0.0/0    scram-sha-256
```

Restart PostgreSQL:

```bash
sudo systemctl restart postgresql

# Verify it's running
systemctl status postgresql
```

## Part 4: Copy CA Certificate to Client (on your laptop)

Copy `ca.crt` from the server to your laptop. The client needs this to verify the server's certificate.

```bash
# From your laptop, copy the CA certificate
scp user@96.126.108.62:~/pg_certs/ca.crt /home/slzatz/vimango/ca.crt

# Set permissions
chmod 600 /home/slzatz/vimango/ca.crt
```

## Part 5: Configure vimango (on your laptop)

### Option A: Using config.json

Edit `config.json`:

```json
{
  "postgres": {
    "host": "96.126.108.62",
    "port": "5432",
    "user": "slzatz",
    "password": "",
    "db": "vimango",
    "ssl_mode": "verify-ca",
    "ssl_ca_cert": "/home/slzatz/vimango/ca.crt"
  }
}
```

### Option B: Using environment variables

Add to `~/.secrets`:

```bash
export VIMANGO_PG_PASSWORD="your_password"
export VIMANGO_PG_SSL_MODE="verify-ca"
export VIMANGO_PG_SSL_CA_CERT="/home/slzatz/vimango/ca.crt"
```

Source it from `~/.bashrc`:

```bash
[[ -f ~/.secrets ]] && source ~/.secrets
```

## SSL Mode Options

| Mode | Encryption | Verifies Certificate | Requires SAN |
|------|------------|---------------------|--------------|
| `disable` | No | No | No |
| `require` | Yes | No | No |
| `verify-ca` | Yes | Yes (signed by trusted CA) | No |
| `verify-full` | Yes | Yes (CA + hostname/IP match) | Yes |

**Note**: The certificate generated in Part 1 uses the legacy CN field, which supports up to `verify-ca` mode. Modern TLS libraries require Subject Alternative Names (SANs) for `verify-full` mode. See [Upgrading to verify-full Mode](#upgrading-to-verify-full-mode) to enable this.

## Troubleshooting

### "Connection refused"
PostgreSQL likely failed to start. Check logs:
```bash
sudo journalctl -u postgresql --since "10 minutes ago"
```

### "Could not load server certificate file"
- Check file permissions: `server.key` must be `600`
- Check ownership: files must be owned by `postgres` user
- Check paths in `postgresql.conf` are absolute and correct

### "Certificate verify failed"
- Ensure `ca.crt` on the client matches the CA that signed `server.crt`
- Check the path to `ssl_ca_cert` in vimango config is correct

### "certificate relies on legacy Common Name field, use SANs instead"
This error occurs when using `verify-full` mode with a certificate that lacks Subject Alternative Names (SANs). The certificate generated in Part 1 uses only the CN field. See [Upgrading to verify-full Mode](#upgrading-to-verify-full-mode) to regenerate the certificate with SANs.

## Upgrading to verify-full Mode

The certificate generated in Part 1 uses the legacy Common Name (CN) field, which modern TLS libraries reject for hostname/IP verification. To enable `verify-full` mode, regenerate the server certificate with a Subject Alternative Name (SAN) extension.

**The CA certificate does not need to change** - only the server certificate needs to be regenerated.

On the server:

```bash
cd ~/pg_certs

# 1. Generate new server key and CSR with SAN extension
#    Replace the IP with your server's IP address
openssl req -new -nodes \
  -out server.csr -keyout server.key \
  -subj "/CN=96.126.108.62" \
  -addext "subjectAltName=IP:96.126.108.62"

# 2. Sign it with your existing CA (note: -copy_extensions copies the SAN)
openssl x509 -req -in server.csr -days 730 \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -copy_extensions copy \
  -out server.crt

# 3. Reinstall certificates for PostgreSQL
sudo cp server.crt server.key /etc/postgresql/ssl/
sudo chown postgres:postgres /etc/postgresql/ssl/server.key /etc/postgresql/ssl/server.crt
sudo chmod 600 /etc/postgresql/ssl/server.key
sudo systemctl restart postgresql
```

On the client, update `ssl_mode` in config.json or environment:

```json
"ssl_mode": "verify-full"
```

The client-side `ca.crt` does not need to change.

### verify-ca vs verify-full

| Mode | What it checks |
|------|----------------|
| `verify-ca` | Certificate is signed by trusted CA |
| `verify-full` | Certificate is signed by trusted CA AND the hostname/IP matches the certificate |

`verify-full` provides additional protection against man-in-the-middle attacks where an attacker might have a valid certificate signed by the same CA but for a different host.

## Certificate Renewal

Server certificate expires in 2 years. To renew, use the appropriate method based on your SSL mode.

### For verify-ca mode (legacy CN approach)

```bash
cd ~/pg_certs

# Generate new CSR and certificate (reuse existing CA)
openssl req -new -nodes \
  -out server.csr -keyout server.key \
  -subj "/CN=96.126.108.62"

openssl x509 -req -in server.csr -days 730 \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt

# Reinstall
sudo cp server.crt server.key /etc/postgresql/ssl/
sudo chown postgres:postgres /etc/postgresql/ssl/server.key /etc/postgresql/ssl/server.crt
sudo chmod 600 /etc/postgresql/ssl/server.key
sudo systemctl restart postgresql
```

### For verify-full mode (with SAN)

```bash
cd ~/pg_certs

# Generate new CSR and certificate with SAN (reuse existing CA)
openssl req -new -nodes \
  -out server.csr -keyout server.key \
  -subj "/CN=96.126.108.62" \
  -addext "subjectAltName=IP:96.126.108.62"

openssl x509 -req -in server.csr -days 730 \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -copy_extensions copy \
  -out server.crt

# Reinstall
sudo cp server.crt server.key /etc/postgresql/ssl/
sudo chown postgres:postgres /etc/postgresql/ssl/server.key /etc/postgresql/ssl/server.crt
sudo chmod 600 /etc/postgresql/ssl/server.key
sudo systemctl restart postgresql
```

The CA certificate (valid 10 years) does not need to change. Client configuration remains the same.
