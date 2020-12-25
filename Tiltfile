docker_build('shard-image', '.', dockerfile='shard/Dockerfile')
docker_build('manager-image', '.', dockerfile='manager/Dockerfile')
docker_build('db-image', '.', dockerfile='db/Dockerfile')
docker_build('rabbit-image', '.', dockerfile='rabbitmq/Dockerfile')
docker_build('api-image', '.', dockerfile='api/Dockerfile')
docker_build('gateway-image', '.', dockerfile='gateway/Dockerfile')
docker_build('dbmanager-image', 'dbmanager', dockerfile='dbmanager/Dockerfile')

k8s_yaml('secret.yaml')
k8s_yaml('shard/shard.yaml')
k8s_yaml('manager/manager.yaml')
k8s_yaml('rabbitmq/rabbit.yaml')
k8s_yaml('db/db.yaml')
k8s_yaml('api/api.yaml')
k8s_yaml('gateway/gateway.yaml')
k8s_yaml('dbmanager/dbmanager.yaml')

k8s_resource('postgres', port_forwards=5432)
k8s_resource('dbmanager')
k8s_resource('shard')
k8s_resource('manager')
k8s_resource('rabbit')
k8s_resource('api', port_forwards=5000)
k8s_resource('gateway', port_forwards=6000)