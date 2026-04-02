const registry = {
  baidu: {
    displayName: '百度热搜',
    aliases: ['baidu'],
    sourceType: 'json-api',
    sourceUrl: 'https://top.baidu.com/api/board?platform=wise&tab=realtime'
  },
  shaoshupai: {
    displayName: '少数派',
    aliases: ['shaoshupai', 'sspai'],
    sourceType: 'json-api',
    sourceUrl: 'https://sspai.com/api/v1/article/index/page/get?limit=20&offset=0&created_at=0'
  },
  weibo: {
    displayName: '微博热搜',
    aliases: ['weibo'],
    sourceType: 'json-api',
    sourceUrl: 'https://weibo.com/ajax/side/hotSearch'
  },
  zhihu: {
    displayName: '知乎热榜',
    aliases: ['zhihu'],
    sourceType: 'json-api',
    sourceUrl: 'https://www.zhihu.com/api/v3/explore/guest/feeds?limit=30&ws_qiangzhisafe=0'
  },
  '36kr': {
    displayName: '36Kr',
    aliases: ['36kr', 'tskr'],
    sourceType: 'json-api',
    sourceUrl: 'https://gateway.36kr.com/api/mis/nav/home/nav/rank/hot'
  },
  '52pojie': {
    displayName: '吾爱破解',
    aliases: ['52pojie', 'ftpojie'],
    sourceType: 'html',
    sourceUrl: 'https://www.52pojie.cn/forum.php?mod=guide&view=hot'
  },
  bilibili: {
    displayName: '哔哩哔哩',
    aliases: ['bilibili'],
    sourceType: 'json-api',
    sourceUrl: 'https://api.bilibili.com/x/web-interface/popular'
  },
  douban: {
    displayName: '豆瓣',
    aliases: ['douban'],
    sourceType: 'html',
    sourceUrl: 'https://www.douban.com/group/explore'
  },
  hupu: {
    displayName: '虎扑',
    aliases: ['hupu'],
    sourceType: 'html',
    sourceUrl: 'https://bbs.hupu.com/all-gambia'
  },
  tieba: {
    displayName: '百度贴吧',
    aliases: ['tieba'],
    sourceType: 'json-api',
    sourceUrl: 'http://tieba.baidu.com/hottopic/browse/topicList'
  },
  juejin: {
    displayName: '掘金',
    aliases: ['juejin'],
    sourceType: 'json-api',
    sourceUrl: 'https://api.juejin.cn/content_api/v1/content/article_rank?category_id=1&type=hot'
  },
  douyin: {
    displayName: '抖音',
    aliases: ['douyin'],
    sourceType: 'json-api',
    sourceUrl: 'https://www.douyin.com/aweme/v1/web/hot/search/list/'
  },
  v2ex: {
    displayName: 'V2EX',
    aliases: ['v2ex', 'vtex'],
    sourceType: 'html',
    sourceUrl: 'https://www.v2ex.com/?tab=hot'
  },
  jinritoutiao: {
    displayName: '今日头条',
    aliases: ['jinritoutiao'],
    sourceType: 'json-api',
    sourceUrl: 'https://www.toutiao.com/hot-event/hot-board/?origin=toutiao_pc'
  },
  tenxunwang: {
    displayName: '腾讯网',
    aliases: ['tenxunwang'],
    sourceType: 'json-api',
    sourceUrl: 'https://i.news.qq.com/gw/event/pc_hot_ranking_list?ids_hash=&offset=0&page_size=51&appver=15.5_qqnews_7.1.60&rank_id=hot'
  },
  stackoverflow: {
    displayName: 'Stack Overflow',
    aliases: ['stackoverflow'],
    sourceType: 'json-api',
    sourceUrl: 'https://api.stackexchange.com/2.3/questions?order=desc&sort=hot&site=stackoverflow'
  },
  github: {
    displayName: 'GitHub Trending',
    aliases: ['github'],
    sourceType: 'json-api',
    sourceUrl: 'https://api.github.com/search/repositories?q=stars:%3E1&sort=stars'
  },
  hackernews: {
    displayName: 'Hacker News',
    aliases: ['hackernews'],
    sourceType: 'html',
    sourceUrl: 'https://news.ycombinator.com/'
  },
  sina_finance: {
    displayName: '新浪财经',
    aliases: ['sina_finance'],
    sourceType: 'json-api',
    sourceUrl: 'https://zhibo.sina.com.cn/api/zhibo/feed?page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20'
  },
  eastmoney: {
    displayName: '东方财富',
    aliases: ['eastmoney'],
    sourceType: 'json-api',
    sourceUrl: 'https://np-weblist.eastmoney.com/comm/web/getFastNewsList'
  },
  xueqiu: {
    displayName: '雪球',
    aliases: ['xueqiu'],
    sourceType: 'json-api',
    sourceUrl: 'https://xueqiu.com/hot_event/list.json?count=10'
  },
  cls: {
    displayName: '财联社',
    aliases: ['cls'],
    sourceType: 'json-api',
    sourceUrl: 'https://www.cls.cn/featured/v1/column/list'
  }
};

const aliasToCanonical = new Map(
  Object.entries(registry).flatMap(([canonical, meta]) =>
    meta.aliases.map((alias) => [alias.toLowerCase(), canonical])
  )
);

/**
 * Resolve canonical platform id from alias.
 * @param {string} input
 * @returns {string | null}
 */
export function normalizePlatformId(input) {
  return aliasToCanonical.get(String(input).trim().toLowerCase()) ?? null;
}

/**
 * Get metadata for a platform or alias.
 * @param {string} input
 * @returns {object | null}
 */
export function getPlatformMeta(input) {
  const canonical = normalizePlatformId(input) ?? input;
  return registry[canonical] ?? null;
}

/**
 * List canonical platform identifiers.
 * @returns {string[]}
 */
export function listPlatforms() {
  return Object.keys(registry);
}

export const platformRegistry = registry;
