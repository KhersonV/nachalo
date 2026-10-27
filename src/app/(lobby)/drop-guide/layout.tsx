import type { ReactNode } from 'react';
import RequireAuth from '@/components/RequireAuth';
import { KnowledgeProvider } from '@/features/knowledge/KnowledgeProvider';

export const metadata = {title: 'Хроники NACHALO'};

export default function KnowledgeLayout({children}:{children:ReactNode}) {
 return <RequireAuth><KnowledgeProvider>{children}</KnowledgeProvider></RequireAuth>;
}
