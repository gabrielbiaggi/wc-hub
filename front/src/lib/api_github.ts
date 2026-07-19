import { api } from '@/lib/api'

export interface GitHubWorkflowRun {
  id: number
  name: string
  display_title: string
  event: string
  status: string
  conclusion: string
  html_url: string
  head_branch: string
  head_sha: string
  run_number: number
  created_at: string
  updated_at: string
}

export interface GitHubRelease {
  id: number
  tag_name: string
  name: string
  html_url: string
  draft: boolean
  prerelease: boolean
  published_at: string
}

export interface GitHubRepository {
  id: number
  full_name: string
  description: string
  default_branch: string
  html_url: string
  private: boolean
  archived: boolean
  updated_at: string
  open_issues_count: number
  stargazers_count: number
  forks_count: number
  size: number
  language: string
  visibility: string
  permissions: { admin: boolean; maintain: boolean; push: boolean; triage: boolean; pull: boolean }
}

export interface GitHubProject {
  repository: GitHubRepository
  workflow_runs: GitHubWorkflowRun[]
  releases: GitHubRelease[]
  error?: string
}

export interface GitHubOverview { generated_at: string; projects: GitHubProject[]; warnings: string[] }

export const getGitHubOverview = async () =>
  (await api.get<GitHubOverview>('/v1/github/overview', { timeout: 45_000 })).data

export const runGitHubWorkflowAction = async (repository: string, runID: number, action: 'rerun' | 'cancel') => {
  const [owner, repo] = repository.split('/', 2)
  return (await api.post<{ status: string }>(`/v1/github/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/actions/runs/${runID}/${action}`, {})).data
}
