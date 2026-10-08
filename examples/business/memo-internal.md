---
title: Q2 Platform Migration Timeline and Team Assignments
subtitle: Engineering Team, Product Management, DevOps
author: Maris Berzins, VP of Engineering
date: 2026-03-28
version: 1.0
status: DRAFT
style: memo
classification: INTERNAL
summary: This memo outlines the Q2 migration plan for moving our core platform from the legacy monolith to the new microservices architecture. All teams must review their assigned deliverables by April 4, 2026.
---

## Background

As discussed in the all-hands on March 20, we are entering the critical phase of our platform modernization initiative. The legacy monolith has served us well for 5 years, but our growth to 200+ tenants has exposed scaling limitations that we can no longer work around.

The board approved a EUR 180,000 budget for Q2 to execute the migration with zero downtime.

## Migration Phases

### Phase 1: Database Layer (April 1--18)

**Lead:** Anna Kalnina (Database Team)

- [ ] Audit all 42 tenant schemas for migration compatibility
- [ ] Set up PostgreSQL 17 cluster with logical replication
- [ ] Create rollback procedures for each migration step
- [x] Complete schema dependency mapping (done March 25)
- [ ] Load test new cluster at 3x current peak traffic

> Critical: No schema changes to production database during April 7--14 freeze window. All pending migrations must be merged by April 4.

### Phase 2: API Gateway (April 14--30)

**Lead:** Edgars Vitols (Backend Team)

Key deliverables:

1. Deploy Kong API Gateway in staging environment
2. Implement rate limiting per tenant (tiered: Basic 100/min, Pro 500/min, Enterprise 2000/min)
3. Configure JWT validation at gateway level
4. Set up request routing for gradual traffic migration
5. Performance benchmark: p99 latency must remain under 50ms

### Phase 3: Service Extraction (May 1--31)

**Lead:** Ieva Ozola (Architecture Team)

Services to extract from monolith:

| Service | Priority | Owner | Target Date |
|---------|----------|-------|-------------|
| Auth Service | P0 | Edgars V. | May 5 |
| Tenant Management | P0 | Anna K. | May 9 |
| Notification Service | P1 | Roberts D. | May 16 |
| File Storage Service | P1 | Kristaps L. | May 16 |
| Reporting Engine | P2 | Ieva O. | May 23 |
| Integration Hub | P2 | Maris B. | May 30 |

### Phase 4: Frontend Migration (May 15--June 15)

**Lead:** Liga Jansone (Frontend Team)

- Migrate from REST polling to WebSocket for real-time dashboard updates
- Implement service worker for offline capability
- Update API client layer to use new gateway endpoints
- A/B test new loading patterns with 10% of users

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Data loss during migration | Low | Critical | Triple backup + verification checksums |
| Extended downtime | Medium | High | Blue-green deployment, instant rollback |
| Performance regression | Medium | Medium | Load testing at each phase gate |
| Team availability (vacation) | High | Medium | Cross-training pairs for each service |

## Budget Allocation

| Category | Amount | Notes |
|----------|--------|-------|
| Infrastructure (new clusters) | EUR 45,000 | Hetzner dedicated + cloud burst |
| Contractor support | EUR 60,000 | 2 senior DevOps for 3 months |
| Testing tools and licenses | EUR 15,000 | k6, Datadog upgrade |
| Contingency (20%) | EUR 36,000 | Board-approved reserve |
| Training and documentation | EUR 24,000 | Team upskilling + runbooks |
| **Total** | **EUR 180,000** | |

## Action Items

1. **All team leads**: Review assigned deliverables and confirm capacity by **April 4**
2. **Anna K.**: Schedule database freeze window communication to all tenants
3. **Edgars V.**: Set up staging environment for API Gateway by **April 7**
4. **Liga J.**: Audit frontend API calls for gateway compatibility
5. **Maris B.**: Finalize contractor onboarding paperwork by **April 1**

## Next Steps

Weekly migration standup begins **April 2** at 10:00 (Riga time), Zoom link in the shared calendar. Status dashboard will be available at the internal Grafana instance under "Migration Tracker."

Questions or concerns should be directed to me or your respective phase lead.
