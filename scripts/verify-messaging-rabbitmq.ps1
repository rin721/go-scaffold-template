param(
    [string]$Image = "rabbitmq:4.3-management"
)

$ErrorActionPreference = "Stop"
$containerName = "go-scaffold-messaging-rabbitmq-43-test"
$testUser = "messaging_integration"
$testPassword = [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(24))

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "docker command is required"
}

$existing = docker ps -a --filter "name=^/$containerName$" --format "{{.Names}}"
if ($LASTEXITCODE -ne 0) {
    throw "failed to inspect Docker containers"
}
if ($existing) {
    throw "test container already exists: $containerName"
}

$created = $false
try {
    $containerID = docker run --detach --name $containerName --publish 127.0.0.1::5672 `
        --env "RABBITMQ_DEFAULT_USER=$testUser" --env "RABBITMQ_DEFAULT_PASS=$testPassword" $Image
    if ($LASTEXITCODE -ne 0 -or -not $containerID) {
        throw "failed to start RabbitMQ test container"
    }
    $created = $true

    $ready = $false
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        docker exec $containerName rabbitmq-diagnostics -q ping *> $null
        if ($LASTEXITCODE -eq 0) {
            $ready = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) {
        throw "RabbitMQ test container did not become ready"
    }

    $policy = '{"delivery-limit":2,"dead-letter-exchange":"gst.messaging.dlx","dead-letter-routing-key":"dead","dead-letter-strategy":"at-least-once","overflow":"reject-publish","delayed-retry-type":"all","delayed-retry-min":100,"delayed-retry-max":200}'
    docker exec $containerName rabbitmqctl enable_feature_flag stream_queue *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "failed to enable RabbitMQ stream_queue feature flag"
    }
    docker exec $containerName rabbitmqctl set_policy gst-messaging '^gst\.messaging\.' $policy --priority 100 --apply-to quorum_queues
    if ($LASTEXITCODE -ne 0) {
        throw "failed to install RabbitMQ messaging test policy"
    }

    $mapping = docker port $containerName 5672/tcp
    if ($LASTEXITCODE -ne 0 -or -not $mapping) {
        throw "failed to resolve RabbitMQ test port"
    }
    $port = ($mapping.Trim() -split ':')[-1]
    if ($port -notmatch '^\d+$') {
        throw "unexpected RabbitMQ test port mapping"
    }
    $escapedPassword = [Uri]::EscapeDataString($testPassword)
    $env:RABBITMQ_MESSAGING_URI = "amqp://${testUser}:$escapedPassword@127.0.0.1:$port/"
    docker exec $containerName rabbitmq-diagnostics server_version
    go test -tags=integration ./internal/kernel/app/messaging/rabbitmq -run '^TestRabbitMQProviderIntegration$' -count=1 -v
    if ($LASTEXITCODE -ne 0) {
        throw "RabbitMQ messaging integration test failed"
    }
}
finally {
    Remove-Item Env:RABBITMQ_MESSAGING_URI -ErrorAction SilentlyContinue
    $testPassword = $null
    if ($created) {
        $resolved = docker inspect --format "{{.Name}}" $containerName 2>$null
        if ($LASTEXITCODE -eq 0 -and $resolved -eq "/$containerName") {
            docker rm --force $containerName *> $null
            if ($LASTEXITCODE -ne 0) {
                throw "failed to remove RabbitMQ test container: $containerName"
            }
        }
        else {
            throw "refusing to remove an unverified RabbitMQ test container"
        }
    }
}
