import { test, expect, type Page } from '@playwright/test';

const now = Date.now();
const spans = [
  { id:'root', time:now-4000, signal:'trace', service:'checkout', name:'POST /checkout', trace_id:'trace-123456789', span_id:'root-span', duration_ms:120, status:'error', value:0, attributes:{environment:'production'} },
  { id:'child', time:now-3980, signal:'trace', service:'payments', name:'authorize', trace_id:'trace-123456789', span_id:'child-span', parent_id:'missing-parent', duration_ms:70, status:'error', value:0, attributes:{'span.late':'true','span.link':'async-1',redaction:'applied'} },
  { id:'log', time:now-3970, signal:'log', service:'payments', name:'card declined', trace_id:'trace-123456789', duration_ms:0, status:'error', value:0, attributes:{redaction:'applied'} },
];

async function mock(page: Page, options: {partial?:boolean;fail?:boolean} = {}) {
  page.on('pageerror', error => console.log('PAGE_ERROR', error.message));
  await page.route('**/api/**', async route => {
    const url = new URL(route.request().url());
    if (!url.pathname.startsWith('/api/')) return route.continue();
    if (url.pathname === '/api/session') return route.fulfill({json:{tenant:'browser-fixture',subject:'tester',role:'admin'}});
    if (url.pathname === '/api/events') {
      if (options.fail) return route.fulfill({status:503,json:{error:'fixture failure'}});
      return route.fulfill({json:{events:spans,next_cursor:url.searchParams.get('trace_id')?'':'older-cursor',timestamp:now,partial:!!options.partial}});
    }
    if (url.pathname === '/api/summary') return route.fulfill({json:{events:3,errors:2,services:2,p95_ms:120,buckets:[{time:now-60000,events:1,errors:0},{time:now,events:2,errors:2}],by_service:[]}});
    if (url.pathname === '/api/agents') return route.fulfill({json:[{id:'collector-1',version:'1.2.0',seen:now,collection:'running',metrics:{queue_depth:'7'}}]});
    if (url.pathname === '/api/trace') return route.fulfill({json:{trace_id:'trace-123456789',spans:spans.filter(item=>item.span_id).map((item,index)=>({...item,links:index?['async-1']:[],missing_parent:index===1,late:index===1,critical:true,fan_out:index?0:2,fan_in:index?2:0})),logs:[spans[2]],metrics:[],partial:false,clock_skew_detected:false}});
    if (url.pathname === '/api/topology') return route.fulfill({json:{edges:[{source:'checkout',destination:'payments',calls:120,errors:2,duration_ms:4800}],clusters:{commerce:2},scanned:3000,partial:false,next_cursor:''}});
    if (url.pathname === '/api/query') return route.fulfill({json:{version:'v1',points:[{time:now,value:42,count:2,group:{service:'checkout'},exemplars:['trace-123456789'],histogram:[{upper:50,count:2}]}],comparison:[{time:now-3600000,value:38,count:2}],execution:{rows_scanned:3,bytes_scanned:512,elapsed_ms:2,partial:!!options.partial,partial_reasons:options.partial?['row_budget']:[],step_ms:1000}}});
    if (url.pathname === '/api/dashboards') return route.request().method()==='POST' ? route.fulfill({json:{id:'saved'}}) : route.fulfill({json:[{id:'one',name:'Checkout health v1',path:'/dashboard/logs',range:'3600000',signal:'log',search:'',service:'checkout'}]});
    if (url.pathname === '/api/dashboard-versions') return route.fulfill({json:[{id:'saved',version_id:'v1',created_at:now,name:'Investigation dashboard',range:'3600000',path:'/dashboard',signal:'',search:'',service:'checkout',panels:[]}]});
    if (url.pathname === '/api/pipeline-health') return route.fulfill({json:{configured:true,scope:'deployment',timestamp:now,stages:[{name:'gateway append',state:'healthy',errors:0}]}});
    if (url.pathname === '/api/notification-status') return route.fulfill({json:{provider_configured:false,pending:1,delivered:0,delivery_boundary:'outbox acknowledgement'}});
    if (url.pathname === '/api/alerts') return route.fulfill({json:[]});
    return route.fulfill({status:404,json:{error:'No test fixture'}});
  });
}

test('filter, pagination, trace drilldown, and keyboard dialog navigation', async ({page}, info) => {
  await mock(page); await page.goto('/dashboard/logs');
  await expect(page.getByRole('heading',{name:'Logs'})).toBeVisible();
  await page.getByLabel('Query').fill('declined'); await page.getByRole('button',{name:'Run'}).click(); await expect(page).toHaveURL(/q=declined/);
  await page.getByRole('button',{name:'Older page'}).click(); await expect(page).toHaveURL(/cursor=older-cursor/);
  await page.getByRole('button',{name:'card declined',exact:true}).click(); await expect(page.getByRole('dialog')).toBeVisible();
  await page.keyboard.press('Tab'); await page.keyboard.press('Shift+Tab'); await expect(page.getByRole('button',{name:'Close trace detail'})).toBeFocused();
  await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toBeHidden();
  if (info.project.name==='desktop') await page.screenshot({path:'../docs/workstreams/assets/investigation-desktop.png',fullPage:true});
});

test('saved dashboard and partial analytical state', async ({page}) => {
  await mock(page,{partial:true}); await page.goto('/dashboard');
  await expect(page.getByText('Checkout health v1')).toBeVisible(); await page.getByRole('button',{name:'Save dashboard'}).click(); await expect(page.getByText(/immutable version saved/)).toBeVisible();
  await expect(page.getByRole('button',{name:/^Version /})).toBeVisible();
  await page.goto('/dashboard/metrics'); await expect(page.getByText(/Budget limit: row_budget/)).toBeVisible();
});

test('server topology, histogram, formula, and operational contracts', async ({page}) => {
  await mock(page); await page.goto('/dashboard/map'); await expect(page.getByRole('button',{name:/checkout 120 calls/})).toBeVisible();
  await page.goto('/dashboard/metrics'); await page.getByLabel('Operation').selectOption('histogram'); await page.getByLabel('Formula').fill('A*2'); await page.getByRole('button',{name:'Run query'}).click(); await expect(page.getByText(/A\*2 · histogram/)).toBeVisible();
  await page.goto('/dashboard'); await expect(page.getByText('gateway append')).toBeVisible();
  await page.goto('/dashboard/alerts'); await expect(page.getByText('Pending transitions')).toBeVisible(); await expect(page.getByText('Not configured')).toBeVisible();
});

test('failed state is explicit', async ({page}) => {
  await mock(page,{fail:true}); await page.goto('/dashboard/logs');
  await expect(page.getByRole('alert').filter({hasText:'Query failed'})).toBeVisible();
});

test('mobile investigation layout', async ({page}, info) => {
  test.skip(info.project.name!=='mobile','mobile project'); await mock(page); await page.goto('/dashboard/logs');
  await expect(page.getByLabel('Investigation scope')).toBeVisible(); await expect(page.getByRole('heading',{name:'Logs'})).toBeVisible();
  await page.screenshot({path:'../docs/workstreams/assets/investigation-mobile.png',fullPage:true});
});

test('protected routes do not flash workspace content', async ({page}) => {
  await page.route('**/api/session',async route=>{await new Promise(resolve=>setTimeout(resolve,250));await route.fulfill({status:401,json:{error:'unauthenticated'}})});
  await page.goto('/dashboard');
  await expect(page.getByRole('heading',{name:'Investigation overview'})).toHaveCount(0);
  await expect(page.getByRole('heading',{name:'Open your workspace.'})).toBeVisible();
});

test('brand, docs, URL restoration, reduced motion, and overflow', async ({page}) => {
  const consoleErrors:string[]=[]; page.on('console',message=>{if(message.type()==='error')consoleErrors.push(message.text())});
  await mock(page);
  await page.emulateMedia({reducedMotion:'reduce'});
  await page.goto('/'); await expect(page.getByRole('heading',{name:/Follow the evidence/})).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang','en');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth-document.documentElement.clientWidth)).toBe(0);
  for(const asset of ['/brand/favicon.svg','/brand/sennet-symbol-dark.svg','/brand/sennet-wordmark-light.svg','/brand/sennet-og-lockup.svg']) {
    expect((await page.request.get(asset)).status()).toBe(200);
  }
  await page.goto('/docs/overview-video'); await expect(page.getByRole('heading',{name:'Two-minute overview',level:1})).toBeVisible();
  await expect(page.getByText(/Final footage is not yet available/)).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth-document.documentElement.clientWidth)).toBe(0);
  await page.goto('/dashboard/traces?visualization=transmission&environment=evaluation');
  await expect(page.getByText('Environment filter unavailable')).toBeVisible();
  await expect(page.getByRole('combobox',{name:'Visualization'})).toHaveValue('transmission');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth-document.documentElement.clientWidth)).toBe(0);
  expect(consoleErrors).toEqual([]);
});
