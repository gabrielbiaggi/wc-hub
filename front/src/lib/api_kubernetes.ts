import { api } from '@/lib/api'
export interface KubeMetadata{name:string;namespace:string;uid:string;creationTimestamp:string;labels:Record<string,string>}
export interface KubeCondition{type:string;status:string;reason:string;message:string;lastTransitionTime:string}
export interface KubeNode{metadata:KubeMetadata;status:{conditions:KubeCondition[];nodeInfo:{kubeletVersion:string;osImage:string;architecture:string};capacity:Record<string,string>}}
export interface KubeDeployment{metadata:KubeMetadata;spec:{replicas:number};status:{replicas:number;readyReplicas:number;availableReplicas:number;unavailableReplicas:number}}
export interface KubePod{metadata:KubeMetadata;status:{phase:string;reason:string;message:string;podIP:string;hostIP:string;containerStatuses:Array<{name:string;ready:boolean;restartCount:number}>}}
export interface KubeEvent{metadata:KubeMetadata;type:string;reason:string;message:string;count:number;lastTimestamp:string;regarding?:{kind:string;namespace:string;name:string};involvedObject?:{kind:string;namespace:string;name:string}}
export interface KubernetesOverview{generated_at:string;nodes:KubeNode[];deployments:KubeDeployment[];problem_pods:KubePod[];events:KubeEvent[]}
export const getKubernetesOverview=async()=>(await api.get<KubernetesOverview>('/v1/kubernetes/overview')).data
export const runKubernetesDeploymentAction=async(namespace:string,name:string,action:'scale'|'restart',replicas?:number)=>(await api.post<{status:string}>(`/v1/kubernetes/namespaces/${encodeURIComponent(namespace)}/deployments/${encodeURIComponent(name)}/${action}`,action==='scale'?{replicas}:{})).data
