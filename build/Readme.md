# NOTES

* we are adopting [this](https://github.com/heatseeknyc/relay/commit/552517843322917cfdc278380d9d1e374d17a8fa) pattern to fix docker issue [#6791](https://github.com/docker/docker/issues/6791#issuecomment-72338100)

# Bastion Systemd Dependency Graph Overview

![Dependency Graph](./dependency_subgraph.png)

# Depth0 Units

* env-lock.path starts env-lock.service following systemd path satisfaction (i.e. bastion-env.sh exists)
* env-lock.service executes after ConditionFileNotEmpty and ConditionPathIsReadWrite is true for the bastion-env.sh file
* docker.service must (of course) start prior to all other units.

# Depth1 Units (Dhese are basic services that all other units depend on)

* nsqd.service
* bastion-etcd.service

# Depth2 Units

* connector.service: Provides VPN connection to Opsee.  _Requires relationship with depth1 units (nsqd.service and bastion-etcd.service)_
    - This means that if any D1 unit dies and systemd does not intervene, the associated D2 unit will not stop.
    - This also means that if a D1 unit dies and systemd intervenes, changes will propogate to the connector and bound child services sharing its network stack.
    - We want the connection to opsee to remain open in the case of catastrophic failure so we don't use BindTo.
* nsqdadmin.service _BindsTo_ nsqd.service beacause it links to the nsqd container
* discovery.service _BindsTo_ nsqd.service because it links to the nsqd container
* hacker.service _BindsTo_ nsqd.service because it liks to the nsqd container
* runner.service _BindsTo_ nsqd.service because it links to the nsqd container

# Depth3 Units

* checker.service _BindsTo_ connector.service because it shares its network stack
* register.service _BindsTo_ connector.service because it shares a volume with it
* shovel-discovery.service _BindsTo_ connector.service because it shares its nework stack
* shovel-results.service _BindsTo_ connector.service because it shares its network stack
* aws-command.service _BindsTo_ connector.service because it shares its network stack
