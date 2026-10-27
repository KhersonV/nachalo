'use client';

import React from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { loadKnowledge } from './service';
import type { CodexData } from './types';

type KnowledgeState={data:CodexData|null;loading:boolean;error:string;refresh:()=>void};
const KnowledgeContext=React.createContext<KnowledgeState|null>(null);

// This layout-scoped catalogue survives article navigation. It contains public
// definitions only and is discarded when leaving the section or changing user.
export function KnowledgeProvider({children}:{children:React.ReactNode}) {
 const {user}=useAuth();const [data,setData]=React.useState<CodexData|null>(null);
 const [loading,setLoading]=React.useState(true);const [error,setError]=React.useState('');
 const [revision,setRevision]=React.useState(0);
 const refresh=React.useCallback(()=>setRevision(value=>value+1),[]);
 React.useEffect(()=>{setData(null)},[user?.token]);
 React.useEffect(()=>{
  if(!user?.token){setLoading(false);return}
  const controller=new AbortController();setLoading(true);setError('');
  loadKnowledge(user.token,controller.signal).then(value=>{if(!controller.signal.aborted)setData(value)}).catch(reason=>{if(!controller.signal.aborted)setError(reason instanceof Error?reason.message:'Не удалось открыть Хроники.')}).finally(()=>{if(!controller.signal.aborted)setLoading(false)});
  return ()=>controller.abort();
 },[user?.token,revision]);
 React.useEffect(()=>{window.addEventListener('focus',refresh);return()=>window.removeEventListener('focus',refresh)},[refresh]);
 return <KnowledgeContext.Provider value={{data,loading,error,refresh}}>{children}</KnowledgeContext.Provider>;
}
export function useKnowledge() {
 const value=React.useContext(KnowledgeContext);
 if(!value)throw new Error('KnowledgeProvider is required');
 return value;
}
