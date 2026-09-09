alter table executions
    add column requested_workspace_id text;

create index executions_requested_workspace_status_idx
    on executions (requested_workspace_id, status, updated_at)
    where requested_workspace_id is not null;
