from __future__ import annotations

from typing import Any
from pathlib import Path
import uuid

from fastapi import FastAPI
from pydantic import BaseModel

from content_hub.bootstrap.container import build_container
from content_hub.domain.formatting.models import FormatTarget


class CreateContentRequest(BaseModel):
    title: str
    body: str
    content_format: str = "markdown"


class UpdateContentRequest(BaseModel):
    path: str
    body: str


class ExecuteWorkflowRequest(BaseModel):
    topic: str


class CreateJobRequest(BaseModel):
    topic: str


class CreateTemplateRequest(BaseModel):
    category: str
    name: str
    content: str


class RenameTemplateRequest(BaseModel):
    path: str
    new_name: str


class CopyTemplateRequest(BaseModel):
    path: str
    target_category: str
    new_name: str


class MoveTemplateRequest(BaseModel):
    path: str
    target_category: str


class UpdateConfigRequest(BaseModel):
    llm: dict | None = None
    workflow: dict | None = None
    rewrite: dict | None = None


class SubmitReferenceUrlsRequest(BaseModel):
    urls: list[str]


class SubmitRawContentRequest(BaseModel):
    items: list[dict]


class SubmitHotTopicsRequest(BaseModel):
    items: list[dict]


class ImportBundleRequest(BaseModel):
    bundle: dict
    provider_profile: str
    article_profile: str
    publish_profile: str


class CreateDraftRequest(BaseModel):
    template: str
    meta: dict
    headline: dict
    sections: list[dict]
    conclusion: str = ""
    cta: str = ""
    source_refs: list[dict] = []
    target_platforms: list[str] = []


class RenderTargetRequest(BaseModel):
    platform: str
    template: str
    output_format: str
    variant: str = "default"


class RenderArticleRequest(BaseModel):
    article_id: str
    targets: list[RenderTargetRequest]


class CreateReviewRequest(BaseModel):
    article_id: str
    asset_ids: list[str]
    reviewer: str = ""
    notes: str = ""


class ReviewDecisionRequest(BaseModel):
    reviewer: str = ""
    notes: str = ""


class PublishReviewRequest(BaseModel):
    article_title: str = ""
    account_info: dict | None = None


class CreateWorkspaceArticleRequest(BaseModel):
    source_type: str
    source_payload: dict
    provider_profile: str
    article_profile: str
    publish_profile: str


class UpdateWorkspaceArticleStatusRequest(BaseModel):
    status: str
    notes: str = ""


def create_app(project_root: Path | None = None, settings_override=None) -> FastAPI:
    app = FastAPI(title="Content Hub API", version="0.1.0")
    resolved_project_root = project_root or Path(__file__).resolve().parents[4]
    container = build_container(resolved_project_root, settings_override)

    @app.get("/health")
    async def health() -> dict[str, str]:
        return {"status": "healthy"}

    @app.get("/config")
    async def get_config() -> dict:
        return {
            "llm": {
                "provider": container.settings.llm.provider,
                "model": container.settings.llm.model,
            },
            "workflow": {
                "publish_platform": container.settings.workflow.publish_platform,
                "article_format": container.settings.workflow.article_format,
                "auto_publish": container.settings.workflow.auto_publish,
            },
            "rewrite": {"enabled": container.settings.rewrite.enabled},
        }

    @app.patch("/config")
    async def update_config(request: UpdateConfigRequest) -> dict:
        if request.llm:
            container.settings.llm.provider = request.llm.get(
                "provider", container.settings.llm.provider
            )
            container.settings.llm.model = request.llm.get("model", container.settings.llm.model)
        if request.workflow:
            container.settings.workflow.publish_platform = request.workflow.get(
                "publish_platform", container.settings.workflow.publish_platform
            )
            container.settings.workflow.article_format = request.workflow.get(
                "article_format", container.settings.workflow.article_format
            )
            container.settings.workflow.auto_publish = request.workflow.get(
                "auto_publish", container.settings.workflow.auto_publish
            )
        if request.rewrite:
            container.settings.rewrite.enabled = request.rewrite.get(
                "enabled", container.settings.rewrite.enabled
            )
        config_path = resolved_project_root / "文章中转站" / "config.generated.yaml"
        container.config_service.save_hub_settings(container.settings, config_path)
        return {"saved_to": str(config_path)}

    @app.get("/templates/categories")
    async def template_categories() -> dict:
        return {"data": container.template_service.list_categories()}

    @app.get("/templates")
    async def templates(
        category: str,
        platform: str | None = None,
        tag: str | None = None,
        theme: str | None = None,
        style: str | None = None,
    ) -> dict:
        return {
            "data": [
                item.__dict__
                for item in container.template_service.list_templates(
                    category,
                    platform=platform,
                    tag=tag,
                    theme=theme,
                    style=style,
                )
            ]
        }

    @app.post("/templates")
    async def create_template(request: CreateTemplateRequest) -> dict:
        path = container.template_service.create_template(request.category, request.name, request.content)
        return {"path": str(path)}

    @app.put("/templates/rename")
    async def rename_template(request: RenameTemplateRequest) -> dict:
        path = container.template_service.rename_template(Path(request.path), request.new_name)
        return {"path": str(path)}

    @app.post("/templates/copy")
    async def copy_template(request: CopyTemplateRequest) -> dict:
        path = container.template_service.copy_template(
            Path(request.path),
            request.target_category,
            request.new_name,
        )
        return {"path": str(path)}

    @app.put("/templates/move")
    async def move_template(request: MoveTemplateRequest) -> dict:
        path = container.template_service.move_template(Path(request.path), request.target_category)
        return {"path": str(path)}

    @app.delete("/templates")
    async def delete_template(path: str) -> dict:
        container.template_service.delete_template(Path(path))
        return {"deleted": True}

    @app.get("/content")
    async def content_list(title: str | None = None, published: bool | None = None) -> dict:
        return {
            "data": [
                {
                    "title": item["title"],
                    "content_format": item["content_format"],
                    "artifact_path": str(item["artifact_path"]),
                    "publish_count": item["publish_count"],
                    "published": item["published"],
                    "last_publish_platform": item["last_publish_platform"],
                    "last_published_at": item["last_published_at"],
                }
                for item in container.content_service.list_document_views(
                    title_query=title,
                    published=published,
                )
            ]
        }

    @app.get("/content/detail")
    async def content_detail(path: str) -> dict:
        detail = container.content_service.get_document_detail(Path(path))
        document = detail["document"]
        return {
            "title": document.title,
            "body": document.body,
            "content_format": document.content_format,
            "artifact_path": str(detail["artifact_path"]),
            "publish_history": detail["publish_history"],
        }

    @app.post("/content")
    async def create_content(request: CreateContentRequest) -> dict:
        artifact = container.content_service.create_document(
            title=request.title,
            body=request.body,
            content_format=request.content_format,
        )
        return {
            "title": artifact.document.title,
            "artifact_path": str(artifact.artifact_path) if artifact.artifact_path else None,
        }

    @app.get("/content/read")
    async def read_content(path: str) -> dict:
        detail = container.content_service.get_document_detail(Path(path))
        document = detail["document"]
        return {
            "title": document.title,
            "body": document.body,
            "content_format": document.content_format,
            "artifact_path": str(detail["artifact_path"]),
            "publish_history": detail["publish_history"],
        }

    @app.put("/content")
    async def update_content(request: UpdateContentRequest) -> dict:
        document = container.content_service.update_document(Path(request.path), request.body)
        return {"title": document.title, "body": document.body}

    @app.delete("/content")
    async def delete_content(path: str) -> dict:
        container.content_service.delete_document(Path(path))
        return {"deleted": True}

    @app.get("/platforms")
    async def list_platforms() -> dict:
        return {"data": container.platform_service.list_platforms()}

    @app.get("/publish/records")
    async def publish_records(article_title: str | None = None) -> dict:
        if article_title:
            return {"data": container.publish_service.get_history(article_title)}
        return {"data": container.publish_service.list_records()}

    @app.post("/drafts")
    async def create_draft(request: CreateDraftRequest) -> dict:
        draft = container.draft_service.create_draft(
            template=request.template,
            meta=request.meta,
            headline=request.headline,
            sections=request.sections,
            conclusion=request.conclusion,
            cta=request.cta,
            source_refs=request.source_refs,
            target_platforms=request.target_platforms,
        )
        return {"article_id": draft.article_id, "status": draft.status, "template": draft.template}

    @app.get("/drafts/{article_id}")
    async def get_draft(article_id: str) -> dict:
        draft = container.draft_service.get_draft(article_id)
        if draft is None:
            return {"article_id": article_id, "status": "missing"}
        return {
            "article_id": draft.article_id,
            "template": draft.template,
            "meta": draft.meta,
            "headline": draft.headline,
            "sections": draft.sections,
            "conclusion": draft.conclusion,
            "cta": draft.cta,
            "source_refs": draft.source_refs,
            "target_platforms": draft.target_platforms,
            "status": draft.status,
        }

    @app.post("/formatting/render")
    async def render_article(request: RenderArticleRequest) -> dict:
        draft = container.draft_service.get_draft(request.article_id)
        if draft is None:
            return {"article_id": request.article_id, "status": "missing"}
        assets = container.formatting_service.format_article(
            draft,
            [
                FormatTarget(
                    platform=target.platform,
                    template=target.template,
                    output_format=target.output_format,
                    variant=target.variant,
                )
                for target in request.targets
            ],
        )
        return {
            "article_id": draft.article_id,
            "assets": [
                {
                    "asset_id": asset.asset_id,
                    "platform": asset.platform,
                    "output_format": asset.output_format,
                    "template": asset.template,
                    "status": asset.status,
                }
                for asset in assets
            ],
        }

    @app.get("/formatting/assets")
    async def list_rendered_assets(article_id: str | None = None, platform: str | None = None) -> dict:
        assets = container.formatting_service.list_assets(article_id=article_id, platform=platform)
        return {
            "data": [
                {
                    "asset_id": asset.asset_id,
                    "article_id": asset.article_id,
                    "platform": asset.platform,
                    "output_format": asset.output_format,
                    "template": asset.template,
                    "status": asset.status,
                }
                for asset in assets
            ]
        }

    @app.get("/formatting/assets/{asset_id}")
    async def get_rendered_asset(asset_id: str) -> dict:
        asset = container.formatting_service.get_asset(asset_id)
        if asset is None:
            return {"asset_id": asset_id, "status": "missing"}
        return {
            "asset_id": asset.asset_id,
            "article_id": asset.article_id,
            "platform": asset.platform,
            "output_format": asset.output_format,
            "template": asset.template,
            "content": asset.content,
            "warnings": asset.warnings,
            "status": asset.status,
        }

    @app.post("/reviews")
    async def create_review(request: CreateReviewRequest) -> dict:
        review = container.review_service.create_review(
            article_id=request.article_id,
            asset_ids=request.asset_ids,
            reviewer=request.reviewer,
            notes=request.notes,
        )
        return {"review_id": review.review_id, "status": review.status, "asset_ids": review.asset_ids}

    @app.post("/reviews/{review_id}/approve")
    async def approve_review(review_id: str, request: ReviewDecisionRequest) -> dict:
        review = container.review_service.approve_review(review_id, reviewer=request.reviewer, notes=request.notes)
        return {"review_id": review.review_id, "status": review.status, "reviewer": review.reviewer}

    @app.post("/reviews/{review_id}/reject")
    async def reject_review(review_id: str, request: ReviewDecisionRequest) -> dict:
        review = container.review_service.reject_review(review_id, reviewer=request.reviewer, notes=request.notes)
        return {"review_id": review.review_id, "status": review.status, "reviewer": review.reviewer}

    @app.post("/reviews/{review_id}/publish")
    async def publish_review(review_id: str, request: PublishReviewRequest) -> dict:
        review = container.review_service.get_review(review_id)
        if review is None:
            return {"review_id": review_id, "status": "missing"}
        assets = [
            asset
            for asset_id in review.asset_ids
            for asset in [container.formatting_service.get_asset(asset_id)]
            if asset is not None
        ]
        article_title = request.article_title or review.article_id
        results = container.publish_gate_service.publish_reviewed_assets(
            review,
            assets,
            article_title=article_title,
            account_info=request.account_info,
        )
        return {
            "review_id": review.review_id,
            "status": review.status,
            "results": [
                {
                    "platform": result.platform,
                    "success": result.success,
                    "message": result.message,
                    "metadata": result.metadata,
                }
                for result in results
            ],
        }

    @app.post("/workspace/articles")
    async def create_workspace_article(request: CreateWorkspaceArticleRequest) -> dict:
        source_payload = request.source_payload if isinstance(request.source_payload, dict) else {}
        title = str(source_payload.get("title") or request.source_type or "workspace-article")
        article = container.workspace_article_service.create_article(
            article_id=f"wa-{uuid.uuid4().hex[:12]}",
            title=title,
        )
        return {
            "article_id": article.article_id,
            "title": article.title,
            "status": article.status,
            "status_history": article.status_history,
        }

    @app.get("/workspace/articles")
    async def list_workspace_articles(status: str | None = None) -> dict:
        return {
            "data": [
                {
                    "article_id": article.article_id,
                    "title": article.title,
                    "status": article.status,
                    "status_history": article.status_history,
                }
                for article in container.workspace_article_service.list_articles(status=status)
            ]
        }

    @app.get("/workspace/articles/{article_id}")
    async def get_workspace_article(article_id: str) -> dict:
        article = container.workspace_article_service.get_article(article_id)
        if article is None:
            return {"article_id": article_id, "status": "missing"}
        return {
            "article_id": article.article_id,
            "title": article.title,
            "status": article.status,
            "status_history": article.status_history,
        }

    @app.post("/workspace/articles/{article_id}/status")
    async def update_workspace_article_status(article_id: str, request: UpdateWorkspaceArticleStatusRequest) -> dict:
        service = container.workspace_article_service
        status = request.status
        article: Any

        if status == "draft_ready":
            mark_draft_ready = getattr(service, "mark_draft_ready", None)
            if callable(mark_draft_ready):
                article = mark_draft_ready(article_id, draft_id="")
            else:
                current = service.get_article(article_id)
                if current is None:
                    raise KeyError(f"workspace article not found: {article_id}")
                if current.status == "draft":
                    article = current
                elif current.status == "review_rejected":
                    article = service.transition_article(article_id, "draft")
                else:
                    raise ValueError(
                        "draft_ready fallback supports only draft or review_rejected"
                    )
        elif status == "rendered":
            mark_rendered = getattr(service, "mark_rendered", None)
            if callable(mark_rendered):
                article = mark_rendered(article_id, asset_ids=[])
            else:
                article = service.transition_article(article_id, "rendered")
        elif status == "review_pending":
            mark_review_pending = getattr(service, "mark_review_pending", None)
            if callable(mark_review_pending):
                article = mark_review_pending(article_id, review_id="")
            else:
                article = service.transition_article(article_id, "review_pending")
        elif status == "approved":
            mark_approved = getattr(service, "mark_approved", None)
            if callable(mark_approved):
                article = mark_approved(article_id, notes=request.notes)
            else:
                article = service.transition_article(article_id, "approved")
        elif status == "published":
            mark_published = getattr(service, "mark_published", None)
            if callable(mark_published):
                article = mark_published(article_id, publish_result_ids=[])
            else:
                article = service.transition_article(article_id, "published")
        elif status == "failed":
            mark_failed = getattr(service, "mark_failed", None)
            if callable(mark_failed):
                article = mark_failed(article_id, notes=request.notes or "manual")
            else:
                current = service.get_article(article_id)
                if current is None:
                    raise KeyError(f"workspace article not found: {article_id}")
                if current.status == "review_pending":
                    article = service.transition_article(article_id, "review_rejected")
                else:
                    raise ValueError(
                        "failed fallback supports only transition from review_pending to review_rejected"
                    )
        else:
            raise ValueError(f"unsupported workspace article status: {status}")

        return {
            "article_id": article.article_id,
            "title": article.title,
            "status": article.status,
            "status_history": article.status_history,
        }

    @app.post("/ingestion/reference-urls")
    async def submit_reference_urls(request: SubmitReferenceUrlsRequest) -> dict:
        return container.ingestion_service.submit_reference_urls(request.urls)

    @app.post("/ingestion/raw-content")
    async def submit_raw_content(request: SubmitRawContentRequest) -> dict:
        return container.ingestion_service.submit_raw_content(request.items)

    @app.post("/ingestion/hot-topics")
    async def submit_hot_topics(request: SubmitHotTopicsRequest) -> dict:
        return container.ingestion_service.submit_hot_topics(request.items)

    @app.post("/ingestion/import-bundle")
    async def import_bundle(request: ImportBundleRequest) -> dict:
        return container.ingestion_service.import_content_hub_bundle(
            bundle=request.bundle,
            provider_profile=request.provider_profile,
            article_profile=request.article_profile,
            publish_profile=request.publish_profile,
        )

    @app.get("/ingestion")
    async def list_ingestion() -> dict:
        return container.ingestion_service.list_records()

    @app.get("/jobs")
    async def list_jobs() -> dict:
        return {
            "data": [
                {
                    "job_id": job.job_id,
                    "status": job.status,
                    "artifact_path": str(job.artifact_path) if job.artifact_path is not None else None,
                }
                for job in container.job_service.list_jobs()
            ]
        }

    @app.post("/jobs")
    async def create_job(request: CreateJobRequest) -> dict:
        job = container.job_service.run_workflow(
            workflow=container.workflow_service.build_default_workflow(container.settings),
            settings=container.settings,
            payload={"topic": request.topic},
        )
        return {"job_id": job.job_id, "status": job.status}

    @app.get("/jobs/{job_id}")
    async def get_job(job_id: str) -> dict:
        job = container.job_service.job_repository.get(job_id)
        if job is None:
            return {"job_id": job_id, "status": "missing"}
        return {
            "job_id": job.job_id,
            "status": job.status,
            "artifact_path": str(job.artifact_path) if job.artifact_path is not None else None,
        }

    @app.post("/jobs/{job_id}/cancel")
    async def cancel_job(job_id: str) -> dict:
        try:
            job = container.job_service.cancel_job(job_id)
        except KeyError:
            return {"job_id": job_id, "status": "missing"}
        return {"job_id": job.job_id, "status": job.status}

    @app.get("/jobs/{job_id}/events")
    async def get_job_events(job_id: str) -> dict:
        job = container.job_service.job_repository.get(job_id)
        if job is None:
            return {"job_id": job_id, "events": []}
        return {
            "job_id": job.job_id,
            "events": [
                {
                    "status": event.status,
                    "message": event.message,
                    "detail": event.detail,
                }
                for event in job.events
            ],
        }

    @app.post("/workflows/execute")
    async def execute_workflow(request: ExecuteWorkflowRequest) -> dict:
        result = container.workflow_service.run_default_workflow(
            settings=container.settings,
            payload={"topic": request.topic},
        )
        return {
            "title": result.document.title if result.document else "",
            "artifact_path": str(result.artifact_path) if result.artifact_path else None,
            "publish_results": [item.message for item in result.publish_results],
        }

    return app
try:
    app = create_app()
except FileNotFoundError:  # pragma: no cover - runtime environment may not have legacy config
    app = FastAPI(title="Content Hub API", version="0.1.0")
