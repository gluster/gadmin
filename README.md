# gadmin - Gluster's storage admin


## Why?

#### History

Gluster project has relied a lot on ‘glusterd’, its config management tool for anything to do with gluster filesystem, its deployment, its expansion, shrinking, and also extended it to provide more status etc. In other words, glusterd tool was Gluster project’s Day 1, Day 2 and Day 3.

With project expanding, and as other tools emerged to do the config management better, there were many projects to just deal with Gluster’s Day 1 (eg., gdeploy), Gluster’s Day 3 (eg, gstatus, gluster-prometheus plugin etc etc).

Also, for integrations with projects like SMB, NFS-ganesha, the software abstraction concept was broken to keep them separate, and the hooks were provided in glusterd project itself.

### Present

‘Glusterd’ is technically, breathing its last breath, rightly so. Its state machine design is not scaling up to 1000s of nodes, and is not able to handle the 1000s of volumes, which is the need of the day. ‘GlusterD2’ project was started with the intention of solving it, and is in the right track to solve it, using the already proven technologies like etcd, go-lang, RESTful interface, gRPC,  etc. But, even then ‘GlusterD2’ project’s main focus will be to setup and manage ‘Gluster Filesystem’ properly across the cluster.

With the above concept in mind, we can’t extend GlusterD2 project to become defacto storage management layer, even though it is capable of handling it. Major issue is due to the abstraction of what are gluster specific commands, and what are the more storage management commands? ‘gDeploy’ project did help in getting some ansible integrations for gluster’s setup issues, but with its name having ‘deploy’ the project failed to reach the status of default storage management tool.

‘gadmin’ is the project we are proposing to manage everything storage, with its Day 1, Day2 and Day3 operations. This would be the default supported tool for storage management from gluster community. Be it kubernetes, HyperConverged, or Good old storage deployment, if one wants to use gluster storage as backend, using this tool would be the recommended way going forward.



## What?
#### Day 1

All users will have is a bunch of machines. A storage management tool should be able to setup the infra needed to get the gluster filesystem, setup the cluster, create the storage volumes as needed by the user. User may just know the required Storage capacity and high availability guarantee, or (s)he may know just the use-case. The tool should be able to make the right judgement based on the available information and provide a storage volume to user to consume.

It should handle the integrations with access protocols like NFS-ganesha, SMB etc. Also manage the Block Storage, or Object storage interfaces if user asks for it.

#### Day 2

Able to manage snapshot, backup strategy, expand volume, (or shrink it), replace hardware, upgrade the software, etc etc.. Each of this may or may not involve data migration, idea is, end user need not be aware of this.

Tuning the parameters of the volume also is part of the Day 2 operations. 
All of the above should happen regardless of the access type (ie, file, block, object).

#### Day 3

Monitoring and debugging.  Also involves log management, stats collection, AI on collected data etc.




## How?

**TODO:** This will be defined soon :-) Keep watching.
