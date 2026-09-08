create table actors (
    id text primary key check (btrim(id) <> ''),
    display_name text not null,
    kind text not null check (kind in ('human')),
    created_at timestamptz not null default now()
);

create table projects (
    id text primary key check (btrim(id) <> ''),
    name text not null check (btrim(name) <> ''),
    created_at timestamptz not null default now()
);

create table project_memberships (
    project_id text not null references projects(id) on delete restrict,
    actor_id text not null references actors(id) on delete restrict,
    role text not null check (role in ('viewer', 'runner', 'reviewer', 'admin')),
    created_at timestamptz not null default now(),
    primary key (project_id, actor_id)
);

create index project_memberships_actor_id_idx
    on project_memberships (actor_id);

create table run_proposals (
    id text primary key check (btrim(id) <> ''),
    project_id text not null references projects(id) on delete restrict,
    created_by text not null references actors(id) on delete restrict,
    status text not null check (status in (
        'DRAFT',
        'PREFLIGHTED',
        'AWAITING_APPROVAL',
        'APPROVED',
        'SUBMITTING',
        'SUBMISSION_UNKNOWN',
        'RUNNING',
        'SUCCEEDED',
        'FAILED',
        'CANCELLED',
        'REJECTED'
    )),
    created_at timestamptz not null,
    unique (id, project_id),
    foreign key (project_id, created_by)
        references project_memberships(project_id, actor_id) on delete restrict
);

create index run_proposals_project_status_created_idx
    on run_proposals (project_id, status, created_at desc);

create index run_proposals_created_by_idx
    on run_proposals (created_by);

create table run_specifications (
    id text primary key check (btrim(id) <> ''),
    proposal_id text not null references run_proposals(id) on delete restrict,
    revision bigint not null check (revision > 0),
    workflow_name text not null check (btrim(workflow_name) <> ''),
    workflow_revision text not null check (btrim(workflow_revision) <> ''),
    normalization_version text not null check (btrim(normalization_version) <> ''),
    normalized_document bytea not null check (octet_length(normalized_document) > 0),
    created_by text not null references actors(id) on delete restrict,
    created_at timestamptz not null,
    unique (proposal_id, revision),
    unique (proposal_id, id)
);

create index run_specifications_created_by_idx
    on run_specifications (created_by);

create table approvals (
    id text primary key check (btrim(id) <> ''),
    project_id text not null references projects(id) on delete restrict,
    proposal_id text not null,
    specification_id text not null,
    reviewer_id text references actors(id) on delete restrict,
    decision text not null check (decision in ('approved', 'rejected', 'policy_approved')),
    policy_version text not null check (btrim(policy_version) <> ''),
    review_context jsonb not null default '{}'::jsonb,
    decided_at timestamptz not null,
    foreign key (proposal_id, project_id)
        references run_proposals(id, project_id) on delete restrict,
    foreign key (proposal_id, specification_id)
        references run_specifications(proposal_id, id) on delete restrict,
    check (
        (decision = 'policy_approved' and reviewer_id is null)
        or (decision in ('approved', 'rejected') and reviewer_id is not null)
    ),
    unique (id, project_id, proposal_id, specification_id)
);

create index approvals_project_proposal_decided_idx
    on approvals (project_id, proposal_id, decided_at desc);

create index approvals_specification_id_idx
    on approvals (specification_id);

create index approvals_reviewer_id_idx
    on approvals (reviewer_id)
    where reviewer_id is not null;

create table executions (
    id text primary key check (btrim(id) <> ''),
    project_id text not null references projects(id) on delete restrict,
    proposal_id text not null,
    specification_id text not null,
    approval_id text not null,
    status text not null check (status in (
        'APPROVED',
        'SUBMITTING',
        'SUBMISSION_UNKNOWN',
        'RUNNING',
        'SUCCEEDED',
        'FAILED',
        'CANCELLED'
    )),
    external_workspace_id text,
    external_execution_id text,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    foreign key (proposal_id, project_id)
        references run_proposals(id, project_id) on delete restrict,
    foreign key (proposal_id, specification_id)
        references run_specifications(proposal_id, id) on delete restrict,
    foreign key (approval_id, project_id, proposal_id, specification_id)
        references approvals(id, project_id, proposal_id, specification_id) on delete restrict,
    check ((external_workspace_id is null) = (external_execution_id is null)),
    unique (external_workspace_id, external_execution_id)
);

create index executions_project_status_updated_idx
    on executions (project_id, status, updated_at);

create index executions_proposal_id_idx
    on executions (proposal_id);

create index executions_specification_id_idx
    on executions (specification_id);

create index executions_approval_id_idx
    on executions (approval_id);

create table execution_attempts (
    id text primary key check (btrim(id) <> ''),
    execution_id text not null references executions(id) on delete restrict,
    attempt_number bigint not null check (attempt_number > 0),
    correlation_id text not null check (btrim(correlation_id) <> ''),
    status text not null check (status in ('pending', 'accepted', 'unknown', 'failed')),
    last_error_category text,
    started_at timestamptz not null,
    resolved_at timestamptz,
    unique (execution_id, attempt_number),
    unique (correlation_id),
    check (resolved_at is null or resolved_at >= started_at)
);

create index execution_attempts_execution_status_idx
    on execution_attempts (execution_id, status);

create table audit_events (
    sequence_id bigint generated always as identity primary key,
    id text not null unique check (btrim(id) <> ''),
    schema_version integer not null check (schema_version > 0),
    project_id text not null references projects(id) on delete restrict,
    proposal_id text,
    actor_id text references actors(id) on delete restrict,
    actor_kind text not null check (actor_kind in ('human', 'system', 'external')),
    event_type text not null check (btrim(event_type) <> ''),
    object_type text,
    object_id text,
    correlation_id text,
    source_system text,
    source_event_id text,
    source_occurred_at timestamptz,
    recorded_at timestamptz not null default now(),
    metadata jsonb not null default '{}'::jsonb,
    check (
        (actor_kind = 'human' and actor_id is not null)
        or (actor_kind in ('system', 'external') and actor_id is null)
    ),
    check ((source_system is null) = (source_event_id is null)),
    foreign key (proposal_id, project_id)
        references run_proposals(id, project_id) on delete restrict
);

create index audit_events_project_sequence_idx
    on audit_events (project_id, sequence_id);

create index audit_events_proposal_sequence_idx
    on audit_events (proposal_id, sequence_id)
    where proposal_id is not null;

create unique index audit_events_source_event_id_idx
    on audit_events (source_system, source_event_id)
    where source_system is not null and source_event_id is not null;

create index audit_events_actor_id_idx
    on audit_events (actor_id)
    where actor_id is not null;
