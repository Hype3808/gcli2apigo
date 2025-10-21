# Deployment Guide

This guide covers different deployment options for gcli2apigo.

## Docker Deployment (Recommended)

### Quick Start with Docker Compose

1. **Clone the repository:**
   ```bash
   git clone <your-repo-url>
   cd gcli2apigo
   ```

2. **Set up environment:**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Start the service:**
   ```bash
   docker-compose up -d
   ```

4. **Access the dashboard:**
   - Open http://localhost:7860 in your browser
   - Login with your configured password

### Using Pre-built Images

```bash
# Pull the latest image
docker pull ghcr.io/your-username/gcli2apigo:latest

# Run the container
docker run -d \
  --name gcli2apigo \
  -p 7860:7860 \
  -e GEMINI_AUTH_PASSWORD=your-secure-password \
  -v $(pwd)/oauth_creds:/app/oauth_creds \
  ghcr.io/your-username/gcli2apigo:latest
```

## Production Deployment

### Docker Compose with Nginx

1. **Update docker-compose.yml:**
   ```yaml
   version: '3.8'
   services:
     gcli2apigo:
       image: ghcr.io/your-username/gcli2apigo:latest
       environment:
         - GEMINI_AUTH_PASSWORD=${GEMINI_AUTH_PASSWORD}
       volumes:
         - ./oauth_creds:/app/oauth_creds
       networks:
         - internal

     nginx:
       image: nginx:alpine
       ports:
         - "80:80"
         - "443:443"
       volumes:
         - ./nginx.conf:/etc/nginx/nginx.conf:ro
         - ./ssl:/etc/nginx/ssl:ro
       depends_on:
         - gcli2apigo
       networks:
         - internal

   networks:
     internal:
       driver: bridge
   ```

2. **Configure SSL certificates:**
   ```bash
   mkdir ssl
   # Copy your SSL certificates to the ssl directory
   # cert.pem and key.pem
   ```

3. **Start services:**
   ```bash
   docker-compose up -d
   ```

### Kubernetes Deployment

1. **Create namespace:**
   ```bash
   kubectl create namespace gcli2apigo
   ```

2. **Create secret for authentication:**
   ```bash
   kubectl create secret generic gcli2apigo-auth \
     --from-literal=password=your-secure-password \
     -n gcli2apigo
   ```

3. **Apply deployment:**
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: gcli2apigo
     namespace: gcli2apigo
   spec:
     replicas: 2
     selector:
       matchLabels:
         app: gcli2apigo
     template:
       metadata:
         labels:
           app: gcli2apigo
       spec:
         containers:
         - name: gcli2apigo
           image: ghcr.io/your-username/gcli2apigo:latest
           ports:
           - containerPort: 7860
           env:
           - name: GEMINI_AUTH_PASSWORD
             valueFrom:
               secretKeyRef:
                 name: gcli2apigo-auth
                 key: password
           - name: HOST
             value: "0.0.0.0"
           - name: PORT
             value: "7860"
           volumeMounts:
           - name: oauth-creds
             mountPath: /app/oauth_creds
           livenessProbe:
             httpGet:
               path: /health
               port: 7860
             initialDelaySeconds: 30
             periodSeconds: 10
           readinessProbe:
             httpGet:
               path: /health
               port: 7860
             initialDelaySeconds: 5
             periodSeconds: 5
         volumes:
         - name: oauth-creds
           persistentVolumeClaim:
             claimName: gcli2apigo-pvc
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: gcli2apigo-service
     namespace: gcli2apigo
   spec:
     selector:
       app: gcli2apigo
     ports:
     - protocol: TCP
       port: 80
       targetPort: 7860
     type: ClusterIP
   ```

## Manual Deployment

### Prerequisites

- Go 1.21 or later
- Git

### Build and Deploy

1. **Clone and build:**
   ```bash
   git clone <your-repo-url>
   cd gcli2apigo
   go build -o gcli2apigo .
   ```

2. **Create systemd service (Linux):**
   ```ini
   [Unit]
   Description=gcli2apigo - Gemini API Proxy
   After=network.target

   [Service]
   Type=simple
   User=gcli2apigo
   WorkingDirectory=/opt/gcli2apigo
   ExecStart=/opt/gcli2apigo/gcli2apigo
   Restart=always
   RestartSec=5
   Environment=GEMINI_AUTH_PASSWORD=your-secure-password
   Environment=HOST=0.0.0.0
   Environment=PORT=7860

   [Install]
   WantedBy=multi-user.target
   ```

3. **Enable and start service:**
   ```bash
   sudo systemctl enable gcli2apigo
   sudo systemctl start gcli2apigo
   ```

## Environment Configuration

### Required Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GEMINI_AUTH_PASSWORD` | API authentication password | `123456` |

### Optional Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HOST` | Server bind address | `0.0.0.0` |
| `PORT` | Server port | `7860` |
| `OAUTH_CREDS_FOLDER` | OAuth credentials directory | `oauth_creds` |

## Security Considerations

### Production Security

1. **Use strong passwords:**
   ```bash
   # Generate a secure password
   openssl rand -base64 32
   ```

2. **Enable HTTPS:**
   - Use nginx or a load balancer with SSL termination
   - Configure proper SSL certificates

3. **Network security:**
   - Use firewalls to restrict access
   - Consider VPN or private networks for internal access

4. **Container security:**
   - Run containers as non-root user (already configured)
   - Use read-only filesystems where possible
   - Regularly update base images

### Monitoring and Logging

1. **Health checks:**
   ```bash
   # Check service health
   curl http://localhost:7860/health
   ```

2. **Log monitoring:**
   ```bash
   # Docker logs
   docker logs gcli2apigo

   # Kubernetes logs
   kubectl logs -f deployment/gcli2apigo -n gcli2apigo
   ```

3. **Metrics collection:**
   - Monitor API response times
   - Track error rates
   - Monitor resource usage

## Backup and Recovery

### Backup OAuth Credentials

```bash
# Create backup
tar -czf oauth_creds_backup_$(date +%Y%m%d).tar.gz oauth_creds/

# Restore backup
tar -xzf oauth_creds_backup_20231201.tar.gz
```

### Database Migration

If you need to migrate credential data:

```bash
# Export credentials
docker exec gcli2apigo cat /app/oauth_creds/usage_stats.json > usage_stats_backup.json
docker exec gcli2apigo cat /app/oauth_creds/banlist.json > banlist_backup.json

# Import to new instance
docker cp usage_stats_backup.json new_gcli2apigo:/app/oauth_creds/usage_stats.json
docker cp banlist_backup.json new_gcli2apigo:/app/oauth_creds/banlist.json
```

## Troubleshooting

### Common Issues

1. **Port already in use:**
   ```bash
   # Change port in environment
   export PORT=8080
   ```

2. **Permission denied on oauth_creds:**
   ```bash
   # Fix permissions
   chmod 700 oauth_creds
   ```

3. **Container won't start:**
   ```bash
   # Check logs
   docker logs gcli2apigo
   ```

### Performance Tuning

1. **Increase file limits:**
   ```bash
   # In docker-compose.yml
   ulimits:
     nofile:
       soft: 65536
       hard: 65536
   ```

2. **Memory limits:**
   ```yaml
   # In docker-compose.yml
   deploy:
     resources:
       limits:
         memory: 512M
       reservations:
         memory: 256M
   ```

## Support

For deployment issues:
1. Check the logs first
2. Verify environment variables
3. Test network connectivity
4. Open an issue on GitHub with deployment details