load('ext://restart_process', 'docker_build_with_restart')

optional_components = ['feature-gate', 'api']
config.define_bool("rust-hot-reload")
config.define_string_list("enable")

# Helper function to validate that components are valid
def valid_components(components):
    all = {c: True for c in optional_components}
    for c in components:
        if c not in all:
            return False
    return True

cfg = config.parse()
rust_hot_reload = cfg.get('rust-hot-reload', False)
enable_list = cfg.get('enable', [])

# Use a dict of key: True as a set
enabled = {c: True for c in enable_list}
if 'all' in enabled:
    enabled = {c: True for c in optional_components}
if not valid_components(enabled):
    fail("Invalid components specified: " + repr(enable_list)
         + ".\nValid components: " + repr(optional_components + ['all']))

# Core components
# ===============

docker_build('shard-image', '.', dockerfile='shard/Dockerfile', ignore=["*", "!shard/**", "!lib/**"])
docker_build('manager-image', '.', dockerfile='manager/Dockerfile', ignore=["*", "!manager/*", "!lib/**"])
docker_build('db-image', '.', dockerfile='db/Dockerfile', ignore=["*", "!db/*", "!lib/**"])
docker_build('sandbox-image', '.', dockerfile='sandbox/Dockerfile', ignore=["*", "!sandbox/*", "!lib/**"])
docker_build('rabbit-image', '.', dockerfile='rabbitmq/Dockerfile', ignore=["*", "!rabbitmq/*", "!lib/**"])
docker_build('dbmanager-image', 'dbmanager', dockerfile='dbmanager/Dockerfile')

k8s_yaml('secret.yaml')
k8s_yaml('shard/shard.yaml')
k8s_yaml('manager/manager.yaml')
k8s_yaml('rabbitmq/rabbit.yaml')
k8s_yaml('db/db.yaml')
k8s_yaml('sandbox/sandbox.yaml')
k8s_yaml('dbmanager/dbmanager.yaml')

k8s_resource('postgres', port_forwards=5432)
k8s_resource('dbmanager')
k8s_resource('shard')
k8s_resource('sandbox', port_forwards=1337)
k8s_resource('manager')
k8s_resource('rabbit')

# Optional components
# ===================

if 'feature-gate' in enabled:
    if rust_hot_reload:
        # Build locally and then use a simplified Dockerfile that just copies the binary into a container
        # Additionally, use hot reloading where the service process is restarted in-place upon rebuilds
        # From https://docs.tilt.dev/example_go.html
        local_resource('feature-gate-compile', 'cargo build --manifest-path=feature-gate/Cargo.toml',
                       deps=['feature-gate/Cargo.toml', 'feature-gate/Cargo.lock', 'feature-gate/build.rs', 'feature-gate/src'])
        docker_build_with_restart('feature-gate-image', '.', dockerfile='feature-gate/tilt-build/Dockerfile', only=["feature-gate/target/debug/feature-gate"],
                                  entrypoint='/usr/bin/feature-gate', live_update=[sync('feature-gate/target/debug/feature-gate', '/usr/bin/feature-gate')])
    else:
        docker_build('feature-gate-image', '.', dockerfile='feature-gate/Dockerfile', ignore=["*", "!feature-gate/**", "!lib/**"])
    k8s_yaml('feature-gate/feature-gate.yaml')
    k8s_resource('feature-gate')

if 'api' in enabled:
    docker_build('api-image', '.', dockerfile='api/Dockerfile', ignore=["*", "!api/*", "!lib/**"])
    docker_build('gateway-image', '.', dockerfile='gateway/Dockerfile', ignore=["*", "!gateway/**", "!lib/**"])
    k8s_yaml('api/api.yaml')
    k8s_yaml('gateway/gateway.yaml')
    k8s_resource('api', port_forwards=5000)
    k8s_resource('gateway', port_forwards=6000)
