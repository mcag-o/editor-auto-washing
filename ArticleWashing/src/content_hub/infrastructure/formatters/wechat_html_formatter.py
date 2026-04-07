from __future__ import annotations

from urllib.parse import urlparse

from content_hub.application.formatting.article_normalizer import ensure_article_defaults, normalize_paragraphs, normalize_text
from content_hub.domain.formatting.models import ArticleDraft, FormatTarget, RenderedAsset
from content_hub.infrastructure.formatters.template_catalog import FileTemplateCatalog


WECHAT_IMAGE_HOST_PATTERNS = ["mmbiz.qpic.cn", "mmbiz.qlogo.cn", "wx.qlogo.cn"]


class WechatHtmlFormatter:
    def __init__(self, template_catalog: FileTemplateCatalog):
        self.template_catalog = template_catalog
        self.last_warnings: list[str] = []

    def validate(self, article: ArticleDraft, target: FormatTarget) -> list[str]:
        errors: list[str] = []
        warnings: list[str] = []
        template_name = str(target.template or article.template or "").strip()
        templates = self.template_catalog.list_templates()
        if template_name not in templates:
            errors.append(f"template must be one of: {', '.join(templates)}")

        normalized = ensure_article_defaults(article)
        title = str(normalized.meta.get("title", "")).strip()
        digest = str(normalized.meta.get("digest", "")).strip()
        author = str(normalized.meta.get("author", "")).strip()
        headline = normalized.headline or {}

        if not title:
            errors.append("meta.title is required")
        elif len(title) > 32:
            errors.append(f"meta.title must be <= 32 characters, got {len(title)}")
        if not digest:
            errors.append("meta.digest is required")
        elif len(digest) > 128:
            errors.append(f"meta.digest must be <= 128 characters, got {len(digest)}")
        if not author:
            errors.append("meta.author is required")
        if not str(normalized.meta.get("thumb_media_id") or normalized.meta.get("cover_media_id") or "").strip():
            warnings.append("meta.thumb_media_id is missing; draft creation will need upload-image first")
        if not str(headline.get("title", "")).strip():
            errors.append("headline.title is required")
        if not normalize_paragraphs(headline.get("body")):
            errors.append("headline.body is required")

        sections = normalized.sections
        if not isinstance(sections, list) or len(sections) == 0:
            errors.append("sections must be a non-empty array")
        else:
            for section_index, section in enumerate(sections):
                if not str(section.get("cn") or section.get("title") or "").strip():
                    errors.append(f"sections[{section_index}] is missing cn/title")
                for block_index, block in enumerate(section.get("blocks", [])):
                    block_type = block.get("type", "card")
                    if block_type in {"card", "opinion"} and not str(block.get("title", "")).strip():
                        errors.append(f"sections[{section_index}].blocks[{block_index}] is missing title")
                    if block_type == "week-ahead" and not block.get("days"):
                        errors.append(f"sections[{section_index}].blocks[{block_index}] needs days")
                    if block_type == "image":
                        url = str(block.get("url", "")).strip()
                        if url and not self._is_wechat_image_url(url):
                            warnings.append(
                                f"sections[{section_index}].blocks[{block_index}] image URL does not look like WeChat CDN"
                            )

        for image_index, image in enumerate(self._find_content_images(normalized), start=1):
            url = str(image.get("url", "")).strip()
            if url and not self._is_wechat_image_url(url):
                warnings.append(f"content image #{image_index} does not look like WeChat CDN: {url}")

        self.last_warnings = warnings
        return errors

    def validate_rendered_output(self, html_text: str) -> list[str]:
        errors: list[str] = []
        html_without_comments = self._strip_html_comments(html_text)
        if "{{" in html_without_comments or "}}" in html_without_comments:
            errors.append("rendered HTML still contains unresolved placeholders")
        html_size = len(html_text.encode("utf-8"))
        if html_size > 1024 * 1024:
            errors.append(f"rendered HTML exceeds 1MB limit: {html_size} bytes")
        return errors

    def render(self, article: ArticleDraft, target: FormatTarget) -> RenderedAsset:
        normalized = ensure_article_defaults(article)
        template_name = target.template or normalized.template
        template_text = self.template_catalog.read_template(template_name)
        html = self._apply_replacements(
            template_text,
            {
                "DATE_CN": normalize_text(normalized.meta.get("date_cn")),
                "DATE_SHORT": normalize_text(normalized.meta.get("date_short")),
                "SOURCE_COUNT": normalize_text(normalized.meta.get("source_count")),
                "NEWS_COUNT": normalize_text(normalized.meta.get("news_count")),
                "TITLE": normalize_text(normalized.meta.get("title")),
                "SUBTITLE": normalize_text(normalized.meta.get("subtitle", "")),
                "AUTHOR": normalize_text(normalized.meta.get("author")),
                "DIGEST": normalize_text(normalized.meta.get("digest", ""), allow_html=True),
                "HEADLINE_TITLE": normalize_text((normalized.headline or {}).get("title")),
                "HEADLINE_BODY": self._render_headline_body(normalized),
                "HEADLINE_SOURCE": normalize_text((normalized.headline or {}).get("source", "")),
                "HEADLINE_IMAGE": self._render_image_block((normalized.headline or {}).get("image")),
                "BODY_SECTIONS": self._render_sections(normalized),
                "CONCLUSION": normalize_text(normalized.conclusion, allow_html=True),
                "CTA": normalize_text(normalized.cta or "你最关注哪一点？欢迎留言讨论。", allow_html=True),
            },
        )
        return RenderedAsset(
            asset_id=f"{normalized.article_id}-{target.platform}-{target.output_format}",
            article_id=normalized.article_id,
            platform=target.platform,
            output_format=target.output_format,
            template=template_name,
            content=html,
            artifact_path=None,
            warnings=[],
        )

    def _apply_replacements(self, template_text: str, replacements: dict[str, str]) -> str:
        rendered = template_text
        for key, value in replacements.items():
            rendered = rendered.replace(f"{{{{{key}}}}}", value)
        return rendered

    def _strip_html_comments(self, html_text: str) -> str:
        rendered = html_text
        while "<!--" in rendered and "-->" in rendered:
            start = rendered.find("<!--")
            end = rendered.find("-->", start)
            if end == -1:
                break
            rendered = rendered[:start] + rendered[end + 3 :]
        return rendered

    def _is_wechat_image_url(self, url: str) -> bool:
        if not url:
            return False
        try:
            host = urlparse(url).hostname or ""
        except ValueError:
            return False
        host = host.lower()
        return any(pattern in host for pattern in WECHAT_IMAGE_HOST_PATTERNS)

    def _find_content_images(self, article: ArticleDraft) -> list[dict]:
        images: list[dict] = []
        headline = article.headline or {}
        if isinstance(headline.get("image"), dict):
            images.append(headline["image"])
        for section in article.sections or []:
            if isinstance(section.get("image"), dict):
                images.append(section["image"])
            for block in section.get("blocks", []):
                if block.get("type", "card") == "image":
                    images.append(block)
        return images

    def _render_headline_body(self, article: ArticleDraft) -> str:
        template_name = article.template or ""
        if template_name == "studio-brief":
            return "".join(
                f'<p style="font-size: 16px; color: #2f2b27; line-height: 1.95; margin: 0 0 10px;">{normalize_text(paragraph, allow_html=True)}</p>'
                for paragraph in normalize_paragraphs((article.headline or {}).get("body"))
            )
        if template_name == "neo-brutalism":
            return "".join(
                f'<p style="font-size: 16px; color: #111111; line-height: 1.82; margin: 0 0 10px; font-weight: 700;">{normalize_text(paragraph, allow_html=True)}</p>'
                for paragraph in normalize_paragraphs((article.headline or {}).get("body"))
            )
        return "".join(
            f'<p style="font-size: 15px; color: #3f3f3f; line-height: 2; margin: 0 0 8px;">{normalize_text(paragraph, allow_html=True)}</p>'
            for paragraph in normalize_paragraphs((article.headline or {}).get("body"))
        )

    def _render_sections(self, article: ArticleDraft) -> str:
        rendered: list[str] = []
        for section in article.sections or []:
            section_en = normalize_text(section.get("en") or section.get("title_en") or "SECTION")
            section_cn = normalize_text(section.get("cn") or section.get("title") or "分区")
            rendered.append(self._render_section_heading(section_en, section_cn, article.template))
            if section.get("image"):
                rendered.append(self._render_image_block(section.get("image"), article.template))
            for block in section.get("blocks", []):
                rendered.append(self._render_block(block, article.template))
        return "".join(rendered)

    def _render_section_heading(self, section_en: str, section_cn: str, template_name: str) -> str:
        if template_name == "studio-brief":
            return (
                '<section style="background: #ffffff; padding: 26px 22px 10px;">'
                f'<p style="font-size: 11px; color: #887d70; letter-spacing: 3px; text-transform: uppercase; margin: 0 0 8px;">{section_en}</p>'
                f'<p style="font-size: 21px; font-weight: 700; color: #171614; line-height: 1.3; margin: 0;">{section_cn}</p>'
                '</section>'
            )
        if template_name == "neo-brutalism":
            return (
                '<section style="background: #fffdf7; padding: 18px 18px 10px;">'
                '<section style="display: inline-block; padding: 10px 12px 8px; background: #00c2ff; border: 4px solid #111111; box-shadow: 6px 6px 0 #111111;">'
                f'<p style="font-size: 11px; color: #111111; letter-spacing: 2.6px; text-transform: uppercase; margin: 0 0 6px; font-weight: 900;">{section_en}</p>'
                f'<p style="font-size: 22px; font-weight: 900; color: #111111; line-height: 1.15; margin: 0;">{section_cn}</p>'
                '</section></section>'
            )
        return (
            '<section style="background: #f8f7f4; padding: 20px 20px 5px;">'
            f'<p style="font-size: 10px; color: #e94560; letter-spacing: 5px; text-transform: uppercase; margin: 0 0 15px; font-weight: 700;">{section_en} · {section_cn}</p>'
            '</section>'
        )

    def _render_block(self, block: dict, template_name: str) -> str:
        block_type = block.get("type", "card")
        if block_type == "card":
            return self._render_card_block(block, template_name=template_name)
        if block_type == "opinion":
            opinion = {**block, "title": f"编辑观点：{block.get('title', '')}", "source": block.get("source") or "39Claw 编辑部"}
            return self._render_card_block(opinion, highlight=True, template_name=template_name)
        if block_type == "week-ahead":
            return self._render_week_ahead_block(block, template_name=template_name)
        if block_type == "paragraph":
            body = self._render_body_paragraphs(block.get("text") or block.get("body"), margin_bottom="12px", template_name=template_name)
            if template_name == "neo-brutalism":
                return f'<section style="background: #fffdf7; padding: 0 18px 8px;">{body}</section>'
            if template_name == "studio-brief":
                return f'<section style="background: #ffffff; padding: 0 22px 8px;">{body}</section>'
            return f'<section style="background: #ffffff; padding: 0 20px 8px;">{body}</section>'
        if block_type == "image":
            return self._render_image_block(block, template_name)
        if block_type == "quote":
            return self._render_quote_block(block, template_name=template_name)
        if block_type == "takeaways":
            return self._render_takeaways_block(block, template_name=template_name)
        raise ValueError(f"Unsupported block type: {block_type}")

    def _render_card_block(self, block: dict, highlight: bool = False, template_name: str = "") -> str:
        raw_number = str(block.get("number", ""))
        number = raw_number.zfill(2) if raw_number.isdigit() else raw_number or "&nbsp;"
        color = "#e94560" if highlight else "#1a1a2e"
        title = normalize_text(block.get("title"))
        source = normalize_text(block.get("source", ""))
        body = self._render_body_paragraphs(block.get("body"), template_name=template_name)
        if template_name == "studio-brief":
            number_bg = "#c96d44" if highlight else "#d9cfbf"
            number_fg = "#fffaf4" if highlight else "#3d372f"
            return (
                '<section style="background: #ffffff; padding: 0 22px 10px;">'
                '<section style="background: #fbf8f1; border: 1px solid rgba(23,22,20,0.08); border-radius: 16px; padding: 18px 18px 16px;">'
                '<section style="display: flex; align-items: flex-start;">'
                f'<section style="min-width: 34px; width: 34px; height: 34px; background: {number_bg}; color: {number_fg}; font-size: 12px; font-weight: 700; line-height: 34px; text-align: center; border-radius: 999px; margin-right: 14px; flex-shrink: 0; letter-spacing: 0.3px;">{number}</section>'
                '<section style="flex: 1;">'
                f'<p style="font-size: 18px; font-weight: 700; color: #171614; margin: 0 0 10px; line-height: 1.4;">{title}</p>'
                f'{body}'
                f'<p style="font-size: 11px; color: #8b8175; margin: 10px 0 0; letter-spacing: 0.4px;">{source}</p>'
                '</section></section></section></section>'
            )
        if template_name == "neo-brutalism":
            number_bg = "#ff5fa2" if highlight else "#ffffff"
            return (
                '<section style="background: #ffffff; padding: 0 18px 12px;">'
                '<section style="background: #ffffff; border: 4px solid #111111; box-shadow: 8px 8px 0 #111111; padding: 16px 14px 14px;">'
                '<section style="display: flex; align-items: flex-start;">'
                f'<section style="min-width: 38px; width: 38px; height: 38px; background: {number_bg}; color: #111111; font-size: 13px; font-weight: 900; line-height: 38px; text-align: center; margin-right: 12px; flex-shrink: 0; border: 3px solid #111111;">{number}</section>'
                '<section style="flex: 1;">'
                f'<p style="font-size: 18px; font-weight: 900; color: #111111; margin: 0 0 10px; line-height: 1.28;">{title}</p>'
                f'{body}'
                f'<p style="font-size: 11px; color: #111111; margin: 10px 0 0; font-weight: 800; letter-spacing: 0.2px;">{source}</p>'
                '</section></section></section></section>'
            )
        return (
            '<section style="background: #ffffff; margin: 0 0 1px; padding: 20px;">'
            '<section style="display: flex; align-items: flex-start;">'
            f'<section style="min-width: 36px; width: 36px; height: 36px; background: {color}; color: #fff; font-size: 14px; font-weight: 900; line-height: 36px; text-align: center; border-radius: 4px; margin-right: 14px; flex-shrink: 0;">{number}</section>'
            '<section style="flex: 1;">'
            f'<p style="font-size: 17px; font-weight: 800; color: #1a1a2e; margin: 0 0 8px; line-height: 1.4;">{title}</p>'
            f'{body}'
            f'<p style="font-size: 11px; color: #bbb; margin: 8px 0 0;">{source}</p>'
            '</section></section></section>'
        )

    def _render_body_paragraphs(self, value: object | None, margin_bottom: str = "8px", template_name: str = "") -> str:
        font_size = "14px"
        text_color = "#555"
        line_height = "1.9"
        if template_name == "studio-brief":
            font_size = "15px"
            text_color = "#332f2a"
            line_height = "1.95"
        elif template_name == "neo-brutalism":
            font_size = "15px"
            text_color = "#111111"
            line_height = "1.82"
        return "".join(
            f'<p style="font-size: {font_size}; color: {text_color}; line-height: {line_height}; margin: 0 0 {margin_bottom};">{normalize_text(paragraph, allow_html=True)}</p>'
            for paragraph in normalize_paragraphs(value)
        )

    def _render_image_block(self, image: object | None, template_name: str = "") -> str:
        if not isinstance(image, dict):
            return ""
        url = normalize_text(image.get("url"))
        if not url:
            return ""
        caption = normalize_text(image.get("caption") or "配图")
        if template_name == "studio-brief":
            return (
                '<section style="background: #ffffff; padding: 8px 22px 24px; text-align: center;">'
                '<section style="background: #f6f2ea; border-radius: 14px; overflow: hidden; border: 1px solid rgba(23,22,20,0.07);">'
                f'<img src="{url}" style="width: 100%; display: block; margin: 0;" />'
                '<section style="padding: 10px 14px 12px;">'
                f'<p style="font-size: 11px; color: #7f766b; text-align: left; margin: 0; line-height: 1.6;">{caption} · AI 生成</p>'
                '</section></section></section>'
            )
        if template_name == "neo-brutalism":
            return (
                '<section style="background: #fffdf7; padding: 10px 18px 0; text-align: center;">'
                '<section style="background: #ffffff; border: 4px solid #111111; box-shadow: 8px 8px 0 #7dff6b; overflow: hidden;">'
                f'<img src="{url}" style="width: 100%; display: block; margin: 0;" />'
                '<section style="padding: 10px 12px 12px; background: #7dff6b; border-top: 4px solid #111111;">'
                f'<p style="font-size: 11px; color: #111111; text-align: left; margin: 0; line-height: 1.55; font-weight: 900; letter-spacing: 0.3px;">{caption}</p>'
                '</section></section></section>'
            )
        return (
            '<section style="background: #ffffff; padding: 15px 20px; text-align: center;">'
            f'<img src="{url}" style="width: 100%; border-radius: 4px; margin: 0;" />'
            '<p style="font-size: 11px; color: #999; text-align: center; margin: 5px 0 0; font-style: italic;">'
            f'{caption} | AI 生成'
            '</p></section>'
        )

    def _render_week_ahead_block(self, block: dict, template_name: str = "") -> str:
        number = normalize_text(block.get("number", ""))
        title = normalize_text(block.get("title") or "下周前瞻")
        source = normalize_text(block.get("source", ""))
        rows_html = "".join(
            (
                f'<p style="font-size: 14px; color: #111111; line-height: 1.78; margin: 0 0 6px; font-weight: 700;"><strong style="color: #111111;">{normalize_text(row.get("label", ""))}</strong> // {normalize_text(row.get("events", ""), allow_html=True)}</p>'
                if template_name == "neo-brutalism"
                else f'<p style="font-size: 14px; color: #555; line-height: 1.9; margin: 0 0 5px;">🔴 <strong>{normalize_text(row.get("label", ""))}</strong>：{normalize_text(row.get("events", ""), allow_html=True)}</p>'
            )
            for row in block.get("days", [])
        )
        if template_name == "neo-brutalism":
            return (
                '<section style="background: #ffffff; padding: 0 18px 12px;">'
                '<section style="background: #ffffff; border: 4px solid #111111; box-shadow: 8px 8px 0 #00c2ff; padding: 16px 14px 14px;">'
                '<section style="display: flex; align-items: flex-start;">'
                f'<section style="min-width: 38px; width: 38px; height: 38px; background: #ffd84d; color: #111111; font-size: 13px; font-weight: 900; line-height: 38px; text-align: center; margin-right: 12px; flex-shrink: 0; border: 3px solid #111111;">{number}</section>'
                '<section style="flex: 1;">'
                f'<p style="font-size: 18px; font-weight: 900; color: #111111; margin: 0 0 10px; line-height: 1.28;">{title}</p>'
                f'{rows_html}'
                f'<p style="font-size: 11px; color: #111111; margin: 10px 0 0; font-weight: 800;">{source}</p>'
                '</section></section></section></section>'
            )
        return (
            '<section style="background: #ffffff; margin: 0 0 1px; padding: 20px;">'
            '<section style="display: flex; align-items: flex-start;">'
            f'<section style="min-width: 36px; width: 36px; height: 36px; background: #e94560; color: #fff; font-size: 14px; font-weight: 900; line-height: 36px; text-align: center; border-radius: 4px; margin-right: 14px; flex-shrink: 0;">{number}</section>'
            '<section style="flex: 1;">'
            f'<p style="font-size: 17px; font-weight: 800; color: #1a1a2e; margin: 0 0 8px; line-height: 1.4;">{title}</p>'
            f'{rows_html}'
            f'<p style="font-size: 11px; color: #bbb; margin: 8px 0 0;">{source}</p>'
            '</section></section></section>'
        )

    def _render_quote_block(self, block: dict, template_name: str = "") -> str:
        text = normalize_text(block.get("text", ""), allow_html=True)
        attribution = normalize_text(block.get("attribution", ""))
        if template_name == "neo-brutalism":
            return (
                '<section style="background: #ffffff; padding: 0 18px 12px;">'
                '<section style="background: #f5f5f5; border: 4px solid #111111; box-shadow: 8px 8px 0 #ffd84d; padding: 14px 14px 12px;">'
                f'<p style="font-size: 16px; color: #111111; line-height: 1.8; margin: 0 0 8px; font-weight: 800;">{text}</p>'
                f'<p style="font-size: 11px; color: #111111; margin: 0; font-weight: 800;">{attribution}</p>'
                '</section></section>'
            )
        return (
            '<section style="background: #ffffff; padding: 10px 20px 20px;">'
            '<section style="border-left: 3px solid #e94560; padding: 6px 0 6px 14px;">'
            f'<p style="font-size: 15px; color: #3f3f3f; line-height: 1.9; margin: 0 0 8px;">{text}</p>'
            f'<p style="font-size: 11px; color: #999; margin: 0;">{attribution}</p>'
            '</section></section>'
        )

    def _render_takeaways_block(self, block: dict, template_name: str = "") -> str:
        title = normalize_text(block.get("title") or "核心结论")
        items_html = "".join(
            f'<li style="margin: 0 0 8px;">{normalize_text(item, allow_html=True)}</li>'
            for item in block.get("items", [])
        )
        if template_name == "neo-brutalism":
            return (
                '<section style="background: #ffffff; padding: 0 18px 12px;">'
                '<section style="background: #ffd84d; border: 4px solid #111111; box-shadow: 8px 8px 0 #111111; padding: 16px 14px 8px;">'
                f'<p style="font-size: 16px; font-weight: 900; color: #111111; margin: 0 0 12px;">{title}</p>'
                f'<ul style="margin: 0; padding-left: 20px; color: #111111; font-size: 15px; line-height: 1.82; font-weight: 800;">{items_html}</ul>'
                '</section></section>'
            )
        return (
            '<section style="background: #ffffff; padding: 20px;">'
            '<section style="background: #f8f7f4; border: 1px solid rgba(233,69,96,0.12); border-radius: 8px; padding: 18px 18px 10px;">'
            f'<p style="font-size: 16px; font-weight: 800; color: #1a1a2e; margin: 0 0 12px;">{title}</p>'
            f'<ul style="margin: 0; padding-left: 20px; color: #555; font-size: 14px; line-height: 1.9;">{items_html}</ul>'
            '</section></section>'
        )
