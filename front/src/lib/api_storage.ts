import{api}from'@/lib/api'
export interface StorageEntry{name:string;path:string;size:number;directory:boolean;modified_at:string;mime_type?:string}
export const browseStorage=async(path='')=>(await api.get<{items:StorageEntry[];path:string}>('/v1/storage/browse',{params:{path}})).data
export const indexStorage=async(path='',limit=2000)=>(await api.get<{items:StorageEntry[]}>('/v1/storage/index',{params:{path,limit}})).data.items
export const storageStreamURL=(path:string)=>`${api.defaults.baseURL??'/api'}/v1/storage/stream?path=${encodeURIComponent(path)}`
export const createStorageDirectory=async(path:string,name:string)=>(await api.post<StorageEntry>('/v1/storage/directories',{path,name})).data
export const uploadStorageFile=async(path:string,file:File)=>{const body=new FormData();body.append('file',file);return(await api.post<StorageEntry>('/v1/storage/upload',body,{params:{path},timeout:300_000})).data}
export const renameStorageEntry=async(path:string,name:string)=>(await api.patch<StorageEntry>('/v1/storage/entry',{path,name})).data
export const deleteStorageEntry=async(path:string)=>api.delete('/v1/storage/entry',{params:{path}})
