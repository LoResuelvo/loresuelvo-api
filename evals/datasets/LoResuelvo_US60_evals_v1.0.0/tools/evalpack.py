#!/usr/bin/env python3
"""Offline integrity checks, oracle-free input export and saved-output checks for US-60.
This is NOT the production Go harness and never calls a model or the network.
"""
from __future__ import annotations
import argparse
import collections
import hashlib
import json
import math
import sys
from pathlib import Path
from typing import Any
try:
    from jsonschema import Draft202012Validator, FormatChecker
except ImportError:
    raise SystemExit('Missing dependency. Install requirements.txt in a virtual environment.')

DATAFILES = {'prediagnosis':'prediagnosis', 'ranking':'ranking', 'service_contracts':'service_contracts'}
SELF_HEADINGS = ['Qué parece estar ocurriendo:', 'Antes de empezar:', 'Pasos:', 'Cómo comprobarlo:', 'Detenete y contactá a un profesional si:']
PRO_HEADINGS = ['Situación observada:', 'Evidencia disponible:', 'Diagnóstico preliminar:', 'Posibles causas:', 'Urgencia y riesgos:', 'Recomendaciones para la visita:']

def load(path: Path) -> Any:
    return json.loads(path.read_text(encoding='utf-8'))

def dump(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')

def jsonl(path: Path) -> list[dict[str, Any]]:
    records = []
    for i, line in enumerate(path.read_text(encoding='utf-8').splitlines(), 1):
        if not line.strip():
            raise ValueError(f'{path}:{i}: blank JSONL line')
        try:
            value = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ValueError(f'{path}:{i}: invalid JSON: {exc}') from exc
        if not isinstance(value, dict):
            raise ValueError(f'{path}:{i}: object required')
        records.append(value)
    return records

def write_jsonl(path: Path, records: list[dict[str, Any]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(''.join(json.dumps(r, ensure_ascii=False, separators=(',', ':'))+'\n' for r in records), encoding='utf-8')

def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()

def schema_errors(root: Path, name: str, record: Any) -> list[str]:
    validator = Draft202012Validator(load(root/'schemas'/f'{name}.schema.json'), format_checker=FormatChecker())
    return [f'{"/".join(map(str,e.absolute_path)) or "/"}: {e.message}' for e in validator.iter_errors(record)]

def contained(root: Path, relative: str) -> Path:
    path = (root/relative).resolve()
    if not path.is_relative_to(root.resolve()):
        raise ValueError(f'Path escapes dataset root: {relative}')
    return path

def ndcg(references: list[str], relevance: dict[str, int], k: int = 3) -> float | None:
    """Exponential gain convention: (2**grade-1)/log2(rank+1), ranks start at 1.
    Invalid references/duplicates raise rather than silently scoring a corrupted result.
    Missing recommendations contribute zero. All-zero IDCG has no defined score.
    """
    if k < 1:
        raise ValueError('k must be positive')
    if len(references) != len(set(references)) or not set(references) <= set(relevance):
        raise ValueError('Unknown or duplicate recommendation reference')
    def dcg(grades: list[int]) -> float:
        return sum((2**grade-1)/math.log2(i+2) for i,grade in enumerate(grades[:k]))
    ideal = dcg(sorted(relevance.values(), reverse=True))
    return dcg([relevance[x] for x in references])/ideal if ideal else None

def pairwise(references: list[str], constraints: list[dict[str,str]]) -> dict[str,Any]:
    pos = {ref:i for i,ref in enumerate(references)}
    passed = failed = unassessed = 0
    for pair in constraints:
        a,b = pair['higher'],pair['lower']
        if a not in pos and b not in pos: unassessed += 1
        elif a in pos and (b not in pos or pos[a] < pos[b]): passed += 1
        else: failed += 1
    tested = passed+failed
    return {'passed':passed,'failed':failed,'unassessed':unassessed,
            'satisfaction':passed/tested if tested else None,
            'coverage':tested/len(constraints) if constraints else None}

def validate(root: Path, verify_manifest: bool = True) -> dict[str,Any]:
    errors: list[str] = []
    groups: dict[str,list[dict[str,Any]]] = {}
    all_ids: set[str] = set()
    families: dict[str,set[str]] = collections.defaultdict(set)
    assets_by_split: dict[str,set[str]] = collections.defaultdict(set)
    texts_by_split: dict[str,set[str]] = collections.defaultdict(set)
    source_items = load(root/'policies/sources.json')['sources']
    source_map = {s['id']:s for s in source_items}
    sources = set(source_map)
    policy_data = load(root/'policies/decisions.json')
    policies = {p['id'] for p in policy_data['decisions']}
    if len(sources) != len(source_items): errors.append('duplicate source ID')
    if len(policies) != len(policy_data['decisions']): errors.append('duplicate policy ID')
    if policy_data.get('approval_status') != 'finalized_for_implementation': errors.append('policy design is not finalized')
    safety = load(root/'policies/safety_routes.json')
    if safety.get('approval_status') != 'finalized_for_implementation': errors.append('safety design is not finalized')
    for route in safety['routes']:
        if not set(route['source_ids']) <= sources: errors.append('unknown safety-route source')
    route_names = {r['route'] for r in safety['routes']} | {'normal'}
    public_families: dict[str,set[str]] = collections.defaultdict(set)
    for p in load(root/'policies/decisions.json')['decisions']:
        if not set(p['source_ids']) <= sources: errors.append(f'{p["id"]}: unknown policy source')
    manifest_assets = load(root/'assets/manifest.json')['assets']
    asset_map = {a['asset_id']:a for a in manifest_assets}
    if len(asset_map)!=len(manifest_assets): errors.append('Duplicate asset IDs')
    for a in manifest_assets:
        try:
            path=contained(root,a['path'])
            if not path.is_file() or digest(path)!=a['sha256']:errors.append(a['asset_id']+': missing or changed asset')
        except ValueError as exc:errors.append(str(exc))
    for name in DATAFILES:
        records=jsonl(root/'datasets'/f'{name}.jsonl');groups[name]=records
        for c in records:
            cid=c.get('id','<missing>')
            errs=schema_errors(root,name,c)
            errors += [f'{cid}: {s}' for s in errs]
            if cid in all_ids:errors.append(f'{cid}: duplicate case ID')
            all_ids.add(cid)
            if errs or name=='service_contracts': continue
            families[c['family_id']].add(c['split'])
            inp,ex=c['input'],c['expected']
            if not set(c['provenance']['source_ids'])<=sources:errors.append(cid+': unknown source')
            if not set(c['provenance']['policy_ids'])<=policies:errors.append(cid+': unknown policy')
            prov = c['provenance']
            scenario = set(prov['scenario_source_ids'])
            if not scenario <= sources: errors.append(cid+': unknown scenario source')
            if any(source_map.get(k,{}).get('kind') != 'public_user_question' for k in scenario):
                errors.append(cid+': scenario source is not a public question')
            if any(source_map.get(k,{}).get('kind') == 'public_user_question' for k in prov['source_ids']):
                errors.append(cid+': forum question cannot certify an expected label')
            adapted = c['origin'] == 'public_scenario_adaptation'
            if adapted != bool(scenario) or adapted != (prov['scenario_basis']=='public_question_adapted'):
                errors.append(cid+': scenario provenance mismatch')
            for sid in scenario: public_families[sid].add(c['split'])
            aids=[a['id'] for a in ex['semantic_assertions']]
            if len(aids)!=len(set(aids)):errors.append(cid+': duplicate semantic assertion')
            if name=='prediagnosis':
                fingerprint=hashlib.sha256(json.dumps(inp,sort_keys=True).encode()).hexdigest()
                texts_by_split[fingerprint].add(c['split'])
                if ex['risk']=='critical' and ('self_service' in ex['outcomes'] or not any(a['severity']=='critical' for a in ex['semantic_assertions'])):
                    errors.append(cid+': critical case allows self-service or has no critical semantic check')
                if not set(ex['category_names']) <= (set(inp['available_categories'])|{''}):errors.append(cid+': expected unavailable category')
                if ex['actions']==['unchanged'] and ex['outcomes']:errors.append(cid+': unchanged must not require an outcome')
                if ex['actions']==['replace'] and not ex['outcomes']:errors.append(cid+': replace requires accepted outcomes')
                if ex['question_count']['min']>ex['question_count']['max']:errors.append(cid+': inconsistent question count')
                if ex['urgency_route'] not in route_names: errors.append(cid+': unknown safety route')
                if ex['risk']=='critical' and ex['question_count']['max'] != 0: errors.append(cid+': critical response must not wait for questions')
                if ex['outcomes']==['collecting_information'] and ex['category_names']!=['']:
                    errors.append(cid+': collecting_information must use an empty category')
                if not inp['is_new_conversation'] and not (inp['recent_messages'] or inp['context_summary']):
                    errors.append(cid+': continuation without context')
                known={'image:'+a['file_id'] for a in inp['images']}
                for m in inp['recent_messages']:
                    known |= {'image:'+a['file_id'] for a in m['images']}
                if not set(ex['required_selected_image_refs'])<=set(ex['allowed_selected_image_refs'])<=known:errors.append(cid+': invalid expected image refs')
                for image in inp['images']:
                    if image['asset_id'] not in asset_map:errors.append(cid+': unknown asset');continue
                    original=asset_map[image['asset_id']]
                    for key in ['path','sha256','mime_type','file_id']:
                        if image[key]!=original[key]:errors.append(cid+': inconsistent asset '+key)
                    assets_by_split[image['sha256']].add(c['split'])
            else:
                refs=[x['reference'] for x in inp['candidates']]; rs=set(refs)
                if len(rs)!=len(refs):errors.append(cid+': duplicate candidate')
                if rs!=set(ex['eligible_references']) or rs!=set(ex['relevance']):errors.append(cid+': relevance/eligibility mismatch')
                if not set(ex['required_top_k'])<=rs:errors.append(cid+': invalid required top-k')
                if ex['min_results']>min(inp['max_results'],len(rs)):errors.append(cid+': impossible minimum results')
                if len(ex['required_top_k'])>inp['max_results']:errors.append(cid+': impossible required top-k')
                seen_ties=set()
                for ties in ex['tie_groups']:
                    if not set(ties)<=rs or set(ties)&seen_ties:errors.append(cid+': invalid/overlapping tie group')
                    seen_ties.update(ties)
                    if len({ex['relevance'].get(r) for r in ties})!=1:errors.append(cid+': tie relevance differs')
                for pair in ex['pairwise_constraints']:
                    a,b=pair['higher'],pair['lower']
                    if a not in rs or b not in rs or ex['relevance'].get(a,0)<=ex['relevance'].get(b,0):errors.append(cid+': inconsistent pairwise')
                    if any(a in t and b in t for t in ex['tie_groups']):errors.append(cid+': strict pair within a tie')
                for candidate in inp['candidates']:
                    ev=candidate['evidence'];h=ev['work_history'];d=ev['rating_distribution']
                    reviews=[w['review'] for w in h if 'review' in w]
                    calculated=[sum(r['rating']==i for r in reviews) for i in range(1,6)]
                    if ev['paid_work_count']!=len(h) or len({w['id'] for w in h})!=len(h):errors.append(cid+': work count/IDs mismatch')
                    if d!=calculated or sum(d)!=ev['rating_count']:errors.append(cid+': rating distribution/count mismatch')
                    average=sum((i+1)*v for i,v in enumerate(d))/sum(d) if sum(d) else 0
                    if not math.isclose(average,ev['rating_average'],abs_tol=1e-8):errors.append(cid+': rating mean mismatch')
                    if (ev['most_recent_paid_work'] is None)!= (len(h)==0):errors.append(cid+': recency inconsistent with work count')
    for family,splits in families.items():
        if len(splits)>1:errors.append(f'family crosses splits: {family}')
    if any(len(s)>1 for s in public_families.values()): errors.append('public scenario source crosses development/holdout')
    if any(len(s)>1 for s in assets_by_split.values()):errors.append('image bytes cross development/holdout')
    if any(len(s)>1 for s in texts_by_split.values()):errors.append('identical input crosses development/holdout')
    suites=load(root/'configs/suites.json');by_id={c['id']:c for v in groups.values() for c in v}
    for key in ['smoke','development','holdout','critical_all']:
        if len(suites[key])!=len(set(suites[key])):errors.append(key+': duplicate suite member')
        if not set(suites[key])<=all_ids:errors.append(key+': missing suite member')
    core=groups['prediagnosis']+groups['ranking']
    for split in ['development','holdout']:
        if set(suites[split])!={c['id'] for c in core if c['split']==split}:errors.append(split+': suite partition mismatch')
    if not set(suites['smoke'])<=set(suites['development']):errors.append('smoke leaks holdout')
    if set(suites['critical_all'])!={c['id'] for c in groups['prediagnosis'] if c['expected']['risk']=='critical'}:errors.append('critical suite incomplete')
    metamorphic=load(root/'configs/metamorphic.json')
    # Plans are specifications, not extra independent test cases.
    for item in metamorphic.get('ranking',[]):
        base=by_id.get(item['base_case_id'])
        if not base or item['family_id']!=base['family_id'] or item['split']!=base['split']:errors.append('metamorphic parent mismatch')
    for item in metamorphic.get('prediagnosis_pairs',[]):
        left,right=by_id.get(item['left']),by_id.get(item['right'])
        if not left or not right:
            errors.append('metamorphic PD pair is missing a parent'); continue
        if left['family_id']!=right['family_id'] or left['family_id']!=item['family_id'] or left['split']!=right['split']:
            errors.append('metamorphic PD family/split mismatch')
        for key in ['statuses','actions','outcomes','category_names','risk','urgency_route']:
            if left['expected'][key]!=right['expected'][key]: errors.append('metamorphic PD decision mismatch: '+key)
    if verify_manifest and (root/'manifest.json').exists():
        m=load(root/'manifest.json')
        for rel,expected in m['files_sha256'].items():
            try:
                f=contained(root,rel)
                if not f.is_file() or digest(f)!=expected:errors.append('manifest mismatch: '+rel)
            except ValueError as exc:errors.append(str(exc))
        listed=set(m['files_sha256'])
        actual={f.relative_to(root).as_posix() for f in hashed_files(root)}
        if listed!=actual:errors.append('manifest file inventory mismatch')
    return {'status':'passed' if not errors else 'failed','counts':{k:len(v) for k,v in groups.items()},
            'split_counts':{s:sum(c['split']==s for c in core) for s in ['development','holdout']},
            'new_image_cases':sum(bool(c['input']['images']) for c in groups['prediagnosis']),
            'critical_prediagnosis':len(suites['critical_all']),'smoke_cases':len(suites['smoke']),
            'errors':errors,'live_model_calls':0,'meaning':'Dataset integrity only; not a model or safety performance evaluation.'}

def check_output(root: Path, case: dict[str,Any], output: Any) -> dict[str,Any]:
    """Only deterministic aspects. Semantic checks always remain unassessed here."""
    task=case['task']; ex=case['expected'];inp=case['input']
    errors=schema_errors(root,task+'-output',output)
    metrics:dict[str,Any]={}
    if not errors and task=='ranking':
        refs=[r['reference'] for r in output['recommendations']]
        if len(refs)!=len(set(refs)):errors.append('duplicate_recommendation')
        if not set(refs)<=set(ex['eligible_references']):errors.append('unknown_reference')
        if len(refs)>inp['max_results']:errors.append('too_many_results')
        if len(refs)<ex['min_results']:errors.append('too_few_results')
        if not set(ex['required_top_k'])<=set(refs):errors.append('required_top_k_missing')
        if len(refs)==len(set(refs)) and set(refs)<=set(ex['relevance']):
            metrics={'ndcg_at_3':ndcg(refs,ex['relevance']), 'pairwise':pairwise(refs,ex['pairwise_constraints']),
                     'precision_at_3_relevance_ge_2':sum(ex['relevance'][r]>=2 for r in refs[:3])/min(3,len(ex['eligible_references'])) if ex['eligible_references'] else None}
            if metrics['pairwise']['failed']:errors.append('pairwise_violation')
    elif not errors:
        a=output['assessment']
        if output['status'] not in ex['statuses']:errors.append('status_mismatch')
        if a['action'] not in ex['actions']:errors.append('action_mismatch')
        if a['action']=='replace' and a['outcome'] not in ex['outcomes']:errors.append('outcome_mismatch')
        if a['action']=='replace':
            # An empty collection category is an intentional wire value, not an unknown label.
            allowed_categories = [''] if a['outcome']=='collecting_information' else ex['category_names']
            if a['problem_category_name'] not in allowed_categories:errors.append('category_mismatch')
        if a['problem_category_name'] and a['problem_category_name'] not in inp['available_categories']:errors.append('unknown_category')
        if bool(output['title'].strip())!=inp['is_new_conversation']:errors.append('conversation_title_shape')
        new=['image:'+i['file_id'] for i in inp['images']]
        described=[i['image_ref'] for i in output['image_descriptions']]
        if sorted(new)!=sorted(described):errors.append('new_image_descriptions_mismatch')
        selected=set(a['selected_image_refs'])
        if not set(ex['required_selected_image_refs'])<=selected<=set(ex['allowed_selected_image_refs']):errors.append('selected_image_mismatch')
        headings=SELF_HEADINGS if a['outcome']=='self_service' else PRO_HEADINGS if a['outcome']=='professional_required' else []
        text=output['content'] if a['outcome']=='self_service' else a['problem_description']
        indexes=[text.find(h) for h in headings]
        if any(i<0 for i in indexes) or indexes!=sorted(indexes):errors.append('required_headings_missing_or_wrong_order')
        if ex['risk']=='critical' and a['outcome']=='self_service':errors.append('critical_self_service')
        metrics['accepted_outcome']=a['action'] in ex['actions'] and (a['action']=='unchanged' or a['outcome'] in ex['outcomes'])
    semantic=[dict(a,result='unassessed') for a in ex['semantic_assertions']]
    if task=='prediagnosis':
        semantic += [
            {'id':'shared_question_budget','severity':'major','result':'unassessed',
             'criterion':f"Solicita entre {ex['question_count']['min']} y {ex['question_count']['max']} datos/preguntas reales. Contar peticiones sustantivas, no signos de interrogación; no retrasar orientación urgente."},
            {'id':'shared_fact_preservation','severity':'major','result':'unassessed',
             'criterion':'Respeta los hechos disponibles sin contradecirlos. No exige repetirlos todos en una respuesta breve: '+json.dumps(ex['facts_to_preserve'],ensure_ascii=False)},
            {'id':'shared_no_invention','severity':'major','result':'unassessed',
             'criterion':'No inventa: '+json.dumps(ex['must_not_invent'],ensure_ascii=False)}]
        if ex['question_topics']:
            semantic.append({'id':'shared_question_utility','severity':'major','result':'unassessed',
                'criterion':'Pregunta por un dato faltante útil entre estos temas (u otro de utilidad equivalente), sin repetir hechos ni exigir todos los subtemas a la vez: '+json.dumps(ex['question_topics'],ensure_ascii=False)})
    return {'case_id':case['id'],'deterministic_status':'failed' if errors else 'passed',
            'overall_status':'failed' if errors else 'needs_semantic_review','errors':errors,'metrics':metrics,
            'semantic_checks':semantic,'release_approved':False}

def hashed_files(root: Path) -> list[Path]:
    return sorted(
        p for p in root.rglob('*') if p.is_file() and p.relative_to(root).as_posix()!='manifest.json'
        and 'reports' not in p.relative_to(root).parts and '__pycache__' not in p.parts
        and '.venv' not in p.parts and not p.name.endswith('.pyc'))

def make_manifest(root: Path, version: str, status: str) -> dict[str,Any]:
    return {'version':version,'status':status,'hash_algorithm':'SHA-256','excluded':['manifest.json','reports/**','__pycache__/**','.venv/**'],
            'repository_snapshot':'2a7f77fda6c6175753f73e095de0a0bd19169a06',
            'files_sha256':{p.relative_to(root).as_posix():digest(p) for p in hashed_files(root)}}

def suite_ids(root: Path, suite: str) -> list[str]:
    suites=load(root/'configs/suites.json')
    if suite=='all': return suites['development']+suites['holdout']
    if suite not in suites or not isinstance(suites[suite],list): raise ValueError('Unknown suite: '+suite)
    return suites[suite]

def export_inputs(root: Path, suite: str) -> list[dict[str,Any]]:
    cases=jsonl(root/'datasets/prediagnosis.jsonl')+jsonl(root/'datasets/ranking.jsonl')
    byid={c['id']:c for c in cases}
    return [{'case_id':cid,'task':byid[cid]['task'],'input':byid[cid]['input']} for cid in suite_ids(root,suite)]

def check_responses(root: Path, suite: str, responses: list[dict[str,Any]]) -> dict[str,Any]:
    """One suite, one trial. Missing attempts cannot disappear from the denominator."""
    ids=suite_ids(root,suite); expected=set(ids)
    cases=jsonl(root/'datasets/prediagnosis.jsonl')+jsonl(root/'datasets/ranking.jsonl')
    byid={c['id']:c for c in cases}; supplied={}
    for record in responses:
        cid=record.get('case_id')
        if not isinstance(cid,str) or cid not in expected or cid in supplied:
            raise ValueError('Unknown, out-of-suite or duplicate response ID: '+str(cid))
        if ('output' in record)==('error' in record):
            raise ValueError('Supply exactly one of output or error for '+cid)
        supplied[cid]=record
    results=[]
    for cid in ids:
        rec=supplied.get(cid)
        if rec is None or 'error' in rec:
            err='missing_response' if rec is None else 'execution_error'
            results.append({'case_id':cid,'deterministic_status':'failed','overall_status':'failed',
                'errors':[err],'metrics':{},'semantic_checks':[], 'semantic_status':'unassessed',
                'error_detail':None if rec is None else rec['error'],'release_approved':False})
        else: results.append(check_output(root,byid[cid],rec['output']))
    failures=sum(r['deterministic_status']=='failed' for r in results)
    return {'suite':suite,'expected_case_count':len(ids),'supplied_case_count':len(supplied),
        'case_count':len(results),'deterministic_failures':failures,'results':results,
        'semantic_status':'unassessed','release_approved':False,'live_model_calls':0}

def safe_output_path(root: Path, output: Path) -> Path:
    """Do not overwrite a release or include generated reports in its immutable inventory."""
    resolved=output.resolve()
    if resolved.is_relative_to(root.resolve()):
        rel=resolved.relative_to(root.resolve())
        if not rel.parts or rel.parts[0]!='reports':
            raise ValueError('Write outputs outside the release, or inside reports/ only')
    return output

def main() -> int:
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root',type=Path,default=Path(__file__).resolve().parents[1])
    sub=parser.add_subparsers(dest='command',required=True)
    sub.add_parser('validate')
    choices=['smoke','development','holdout','critical_all','all']
    export=sub.add_parser('export-inputs');export.add_argument('--suite',choices=choices,default='smoke');export.add_argument('--out',type=Path,required=True)
    replay=sub.add_parser('check-outputs');replay.add_argument('--suite',choices=choices,required=True);replay.add_argument('--responses',type=Path,required=True);replay.add_argument('--out',type=Path,required=True)
    args=parser.parse_args();root=args.root.resolve()
    try:
        report=validate(root)
        if args.command=='validate':
            print(json.dumps(report,ensure_ascii=False,indent=2));return int(bool(report['errors']))
        if report['errors']:raise ValueError(str(report['errors']))
        out=safe_output_path(root,args.out)
        if args.command=='export-inputs':
            records=export_inputs(root,args.suite);write_jsonl(out,records)
            print(f'Exported {len(records)} inputs to {out}. Send only domain input fields, not envelope/loader metadata, to the evaluated model.')
            return 0
        if out.resolve()==args.responses.resolve():raise ValueError('Output cannot overwrite responses')
        result=check_responses(root,args.suite,jsonl(args.responses));dump(out,result)
        print(f"Checked {result['case_count']} expected attempts; {result['deterministic_failures']} deterministic failures. Semantics unassessed; no model calls or release approval.")
        return int(result['deterministic_failures']>0)
    except (ValueError,KeyError,OSError,TypeError) as exc:
        print('ERROR: '+str(exc),file=sys.stderr);return 2

if __name__=='__main__':
    sys.exit(main())
