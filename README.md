## Glusterfs server in kubernetes cluster
Idea very simple, cluster manager listen kubernetes api and add to /etc/hosts "metadata.name" and pod ip address.


### 1. Create pods

gluster1.yaml

```
apiVersion: v1
kind: Pod
metadata:
  name: gluster1
  namespace: mynamespace
  labels:
    component: glusterfs-storage
spec:
  nodeSelector:
    host: st01
  containers:
    - name: glusterfs-server
      image: suquant/glusterd:3.6.kube
      imagePullPolicy: Always
      command:
        - /kubernetes-glusterd
      args:
        - --namespace
        - mynamespace
        - --labels
        - component=glusterfs-storage
      ports:
        - containerPort: 24007
        - containerPort: 24008
        - containerPort: 49152
        - containerPort: 38465
        - containerPort: 38466
        - containerPort: 38467
        - containerPort: 2049
        - containerPort: 111
        - containerPort: 111
          protocol: UDP
      volumeMounts:
        - name: brick
          mountPath: /mnt/brick
        - name: fuse
          mountPath: /dev/fuse
        - name: data
          mountPath: /var/lib/glusterd
      securityContext:
        capabilities:
          add:
            - SYS_ADMIN
            - MKNOD
  volumes:
    - name: brick
      hostPath:
        path: /opt/var/lib/brick1
    - name: fuse
      hostPath:
        path: /dev/fuse
    - name: data
      emptyDir: {}

```

gluster2.yaml

```
apiVersion: v1
kind: Pod
metadata:
  name: gluster2
  namespace: mynamespace
  labels:
    component: glusterfs-storage
spec:
  nodeSelector:
    host: st02
  containers:
    - name: glusterfs-server
      image: suquant/glusterd:3.6.kube
      imagePullPolicy: Always
      command:
        - /kubernetes-glusterd
      args:
        - --namespace
        - mynamespace
        - --labels
        - component=glusterfs-storage
      ports:
        - containerPort: 24007
        - containerPort: 24008
        - containerPort: 49152
        - containerPort: 38465
        - containerPort: 38466
        - containerPort: 38467
        - containerPort: 2049
        - containerPort: 111
        - containerPort: 111
          protocol: UDP
      volumeMounts:
        - name: brick
          mountPath: /mnt/brick
        - name: fuse
          mountPath: /dev/fuse
        - name: data
          mountPath: /var/lib/glusterd
      securityContext:
        capabilities:
          add:
            - SYS_ADMIN
            - MKNOD
  volumes:
    - name: brick
      hostPath:
        path: /opt/var/lib/brick1
    - name: fuse
      hostPath:
        path: /dev/fuse
    - name: data
      emptyDir: {}

```

### 3. Run pods

```sh
kubectl create -f gluster1.yaml
kubectl create -f gluster2.yaml
```

### 2. Manage glusterfs servers

```
kubectl --namespace=mynamespace exec -ti gluster1 -- sh -c "gluster peer probe gluster2"
kubectl --namespace=mynamespace exec -ti gluster1 -- sh -c "gluster peer status"
kubectl --namespace=mynamespace exec -ti gluster1 -- sh -c "gluster volume create media replica 2 transport tcp,rdma gluster1:/mnt/brick gluster2:/mnt/brick force"
kubectl --namespace=mynamespace exec -ti gluster1 -- sh -c "gluster volume start media"
```

### 3. Usage

gluster-svc.yaml
```
kind: Service
apiVersion: v1
metadata:
  name: glusterfs-storage
  namespace: mynamespace
spec:
  ports:
    - name: glusterfs-api
      port: 24007
      targetPort: 24007
    - name: glusterfs-infiniband
      port: 24008
      targetPort: 24008
    - name: glusterfs-brick0
      port: 49152
      targetPort: 49152
    - name: glusterfs-nfs-0
      port: 38465
      targetPort: 38465
    - name: glusterfs-nfs-1
      port: 38466
      targetPort: 38466
    - name: glusterfs-nfs-2
      port: 38467
      targetPort: 38467
    - name: nfs-rpc
      port: 111
      targetPort: 111
    - name: nfs-rpc-udp
      port: 111
      targetPort: 111
      protocol: UDP
    - name: nfs-portmap
      port: 2049
      targetPort: 2049
  selector:
    component: glusterfs-storage
```

Run service
```
kubectl create -f gluster-svc.yaml
```


After you can mount NFS in cluster by hostname "glusterfs-storage.mynamespace"