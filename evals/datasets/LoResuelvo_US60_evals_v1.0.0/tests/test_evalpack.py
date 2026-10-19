"""Offline regression tests for package tools. No model/network requests."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import shutil
import unittest
ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('evalpack', ROOT/'tools/evalpack.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
PD=m.jsonl(ROOT/'datasets/prediagnosis.jsonl')
RK=m.jsonl(ROOT/'datasets/ranking.jsonl')
SM=m.jsonl(ROOT/'datasets/summarization.jsonl')

class PDSharedContractTests(unittest.TestCase):
    def test_shared_rubric_matches_go_contract(self):
        case=copy.deepcopy(PD[0])
        case['expected']['facts_to_preserve']=['Reported action']
        case['expected']['must_not_invent']=['Confirmed cause']
        checks={c['id']:c for c in m.check_output(ROOT,case,None)['semantic_checks']}
        go=(ROOT.parents[2]/'internal/evals/evaluation.go').read_text()
        for key,field in [('shared_fact_preservation','facts_to_preserve'),('shared_no_invention','must_not_invent')]:
            import re
            prefix=re.search(r'add\("'+key+r'", "([^"]*)"',go).group(1)
            self.assertEqual(checks[key]['criterion'],prefix+'; '.join(case['expected'][field]))
            self.assertEqual(checks[key]['result'],'unassessed')
            self.assertEqual(checks[key]['severity'],'major')

class SharedReadabilityTests(unittest.TestCase):
    def test_same_minor_semantic_criterion_in_all_tasks_and_go(self):
        import re
        go=(ROOT.parents[2]/'internal/evals/evaluation.go').read_text()
        criterion=re.search(r'const sharedReadabilityCriterion = "([^\"]*)"',go)
        self.assertIsNotNone(criterion)
        for case in PD+RK+SM:
            for output in (None, {}, {'summary':'A brief paragraph.'} if case['task']=='summarization' else {'recommendations':[]} if case['task']=='ranking' else {'content':'Section: 1. Step. 2. Step.'}):
                with self.subTest(case=case['id'],output=output):
                    result=m.check_output(ROOT,case,output)
                    checks=[c for c in result['semantic_checks'] if c['id']=='shared_readability']
                    self.assertEqual(len(checks),1)
                    self.assertEqual(checks[0],{'id':'shared_readability','severity':'minor','criterion':criterion.group(1),'result':'unassessed'})
                    self.assertFalse(result['release_approved'])

class MetricsTests(unittest.TestCase):
    def test_ndcg_ideal(self):self.assertAlmostEqual(m.ndcg(['a','b','c'],{'a':3,'b':2,'c':1}),1)
    def test_ndcg_wrong_order(self):self.assertLess(m.ndcg(['c','b','a'],{'a':3,'b':2,'c':1}),1)
    def test_ndcg_ties(self):self.assertEqual(m.ndcg(['a','b'],{'a':2,'b':2}),m.ndcg(['b','a'],{'a':2,'b':2}))
    def test_ndcg_zero(self):self.assertIsNone(m.ndcg(['a'],{'a':0}))
    def test_ndcg_missing(self):self.assertLess(m.ndcg(['a'],{'a':3,'b':2}),1)
    def test_ndcg_unknown(self):
        with self.assertRaises(ValueError):m.ndcg(['x'],{'a':1})
    def test_ndcg_duplicate(self):
        with self.assertRaises(ValueError):m.ndcg(['a','a'],{'a':1})
    def test_pairwise_unassessed(self):self.assertEqual(m.pairwise([], [{'higher':'a','lower':'b'}])['unassessed'],1)
    def test_pairwise_missing_lower(self):self.assertEqual(m.pairwise(['a'], [{'higher':'a','lower':'b'}])['passed'],1)
    def test_pairwise_missing_higher(self):self.assertEqual(m.pairwise(['b'], [{'higher':'a','lower':'b'}])['failed'],1)

class ValidationTests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.root=Path(self.tmp.name)/'p'
        for name in ['datasets','schemas','policies','configs','assets']:
            shutil.copytree(ROOT/name,self.root/name)
    def tearDown(self):self.tmp.cleanup()
    def mutate(self,name,fn):
        p=self.root/'datasets'/f'{name}.jsonl';items=m.jsonl(p);fn(items);m.write_jsonl(p,items)
    def has_error(self,needle):self.assertTrue(any(needle in e for e in m.validate(self.root)['errors']),m.validate(self.root))
    def test_package_valid(self):self.assertEqual(m.validate(self.root)['errors'],[])
    def test_duplicate_case(self):
        self.mutate('prediagnosis',lambda x:x.append(copy.deepcopy(x[0])));self.has_error('duplicate case')
    def test_invalid_rating(self):
        self.mutate('ranking',lambda x:x[0]['input']['candidates'][0]['evidence'].__setitem__('rating_average',0.3));self.has_error('rating mean')
    def test_cross_split(self):
        self.mutate('prediagnosis',lambda x:x[7].__setitem__('family_id',x[0]['family_id']));self.has_error('family crosses')
    def test_missing_asset(self):
        (self.root/'assets/fixture_01.png').unlink();self.has_error('missing or changed asset')
    def test_critical_selfservice(self):
        self.mutate('prediagnosis',lambda x:x[26]['expected'].__setitem__('outcomes',['self_service']));self.has_error('critical case')
    def test_unknown_source(self):
        self.mutate('prediagnosis',lambda x:x[0]['provenance']['source_ids'].append('unknown'));self.has_error('unknown source')
    def test_smoke_leak(self):
        p=self.root/'configs/suites.json';s=m.load(p);s['smoke'].append(s['holdout'][0]);m.dump(p,s);self.has_error('smoke leaks')
    def test_extra_schema_field(self):
        self.mutate('prediagnosis',lambda x:x[0].__setitem__('undocumented_field',1));self.has_error('Additional properties')
    def test_manifest_tamper(self):
        m.dump(self.root/'manifest.json',m.make_manifest(self.root,'test','test'))
        with (self.root/'requirements.txt').open('w') as f:f.write('tamper')
        self.has_error('inventory mismatch')
    def test_output_unknown_reference(self):
        r=m.check_output(self.root,RK[0],{'recommendations':[{'reference':'invented','reason':'x'}]})
        self.assertIn('unknown_reference',r['errors'])
    def test_valid_ranking_never_auto_approves_semantics(self):
        c=RK[0];refs=sorted(c['expected']['relevance'],key=c['expected']['relevance'].get,reverse=True)[:3]
        r=m.check_output(self.root,c,{'recommendations':[{'reference':x,'reason':'Test-only synthetic placeholder.'} for x in refs]})
        self.assertEqual(r['deterministic_status'],'passed');self.assertEqual(r['overall_status'],'needs_semantic_review');self.assertFalse(r['release_approved'])
    def test_output_unchanged_cannot_carry_outcome(self):
        out={'status':'out_of_scope','title':'Tema ajeno','content':'No corresponde al servicio.','image_descriptions':[],
        'assessment':{'action':'unchanged','outcome':'self_service','problem_title':'','problem_description':'','problem_category_name':'','selected_image_refs':[]}}
        self.assertTrue(m.schema_errors(self.root,'prediagnosis-output',out))

class FinalDatasetTests(unittest.TestCase):
    def test_full_final_integrity(self):self.assertEqual(m.validate(ROOT)['errors'],[])
    def test_counts_and_approval(self):
        self.assertEqual((len(PD),len(RK),len(SM)),(47,19,4))
        self.assertTrue(all(c['review_status']=='approved' and c['version']=='1.0.0' for c in PD+RK+SM))
    def test_summary_fidelity_remains_unassessed(self):
        r=m.check_output(ROOT,SM[0],{'summary':'A structurally valid but unverified summary.'})
        self.assertEqual(r['deterministic_status'],'passed')
        self.assertEqual(r['overall_status'],'needs_semantic_review')
        self.assertFalse(r['release_approved'])
        self.assertTrue(all(c['result']=='unassessed' and (c['severity']=='minor' if c['id']=='shared_readability' else not c['severity']) for c in r['semantic_checks']))
    def test_summary_exact_image_identity(self):
        c=next(c for c in SM if c['expected']['required_image_refs'])
        ref=c['expected']['required_image_refs'][0]['reference']
        self.assertIn('required_image_reference_missing',m.check_output(ROOT,c,{'summary':ref+'extra'})['errors'])
        self.assertNotIn('required_image_reference_missing',m.check_output(ROOT,c,{'summary':'Photos '+', '.join(a['reference'] for a in c['expected']['required_image_refs'])+'.'})['errors'])
    def test_summary_empty_and_extra_field_rejected(self):
        for out in [{'summary':'  '},{'summary':'x','extra':1}]:
            self.assertEqual(m.check_output(ROOT,SM[0],out)['deterministic_status'],'failed')
    def test_rank_count_is_maximum_not_minimum(self):
        r=m.check_output(ROOT,RK[0],{'recommendations':[]})
        self.assertNotIn('too_few_results',r['errors'])
        self.assertNotIn('required_top_k_missing',r['errors'])
    def test_family_partitions_preserved(self):
        families={}
        for c in PD+RK+SM:families.setdefault(c['family_id'],set()).add(c['split'])
        self.assertTrue(all(len(s)==1 for s in families.values()))
    def test_real_source_is_not_a_label_source(self):
        for c in PD:
            self.assertTrue(set(c['provenance']['scenario_source_ids']).isdisjoint(c['provenance']['source_ids']))
    def test_reviews_have_variation(self):
        comments=[w['review']['description'] for c in RK for x in c['input']['candidates'] for w in x['evidence']['work_history'] if 'review' in w]
        self.assertGreater(len(set(comments)),40)
        self.assertNotIn('Se resolvió el problema descrito.',comments)
    def test_negative_reviews_do_not_claim_success(self):
        bad=[w['review']['description'] for x in RK[15]['input']['candidates'] for w in x['evidence']['work_history'] if 'review' in w and w['review']['rating']==1]
        self.assertTrue(bad)
        self.assertTrue(all(any(token in t.lower() for token in ['sigue','no quedó','no se arregló']) for t in bad))
    def test_collecting_valid_category_is_not_rejected(self):
        c=next(c for c in PD if c['expected']['outcomes']==['collecting_information'] and c['input']['is_new_conversation'])
        out={'status':'answered','title':'Sin agua caliente','content':'¿Qué tipo de equipo tenés? ¿Notaste alguna señal de peligro?', 'image_descriptions':[],
             'assessment':{'action':'replace','outcome':'collecting_information','problem_title':'','problem_description':'','problem_category_name':'','selected_image_refs':[]}}
        result=m.check_output(ROOT,c,out)
        self.assertEqual(result['errors'],[])
        self.assertFalse(result['release_approved'])
        self.assertGreaterEqual(len(result['semantic_checks']),4)
        self.assertTrue(all(x['result']=='unassessed' and x['criterion'] for x in result['semantic_checks']))
    def test_collecting_fabricated_category_rejected(self):
        c=next(c for c in PD if c['expected']['outcomes']==['collecting_information'] and c['input']['is_new_conversation'])
        out={'status':'answered','title':'Sin agua caliente','content':'Revisión.', 'image_descriptions':[],
             'assessment':{'action':'replace','outcome':'collecting_information','problem_title':'','problem_description':'','problem_category_name':'Gas','selected_image_refs':[]}}
        self.assertEqual(m.check_output(ROOT,c,out)['deterministic_status'],'failed')
    def test_export_contains_no_oracle_metadata(self):
        rows=m.export_inputs(ROOT,'development')
        self.assertEqual(len(rows),51)
        for r in rows:
            self.assertEqual(set(r),{'case_id','task','input'})
            self.assertFalse({'expected','provenance','split','family_id','risk','relevance','semantic_assertions'} & set(r['input']))
    def test_ranking_problem_titles_do_not_leak_oracle(self):
        forbidden=['empate','pertinente','igualmente','inyección','reseña','historial','evidencia','mérito','redacción','satisfacción']
        for c in RK:
            title=c['input']['problem_title'].lower()
            self.assertFalse(any(word in title for word in forbidden),c['id'])
    def test_injection_pair_preserves_base_message(self):
        self.assertIn('enchufe tira chispas cada rato aunque no tenga nada conectado',PD[40]['input']['user_message'].lower())
        self.assertIn('enchufe tira chispas cada rato aunque no tenga nada conectado',PD[28]['input']['user_message'].lower())
    def test_all_export_has_seventy(self):self.assertEqual(len(m.export_inputs(ROOT,'all')),70)
    def test_missing_responses_are_failures(self):
        r=m.check_responses(ROOT,'smoke',[])
        self.assertEqual(r['case_count'],17)
        self.assertEqual(r['deterministic_failures'],17)
        self.assertTrue(all('missing_response' in x['errors'] for x in r['results']))
    def test_explicit_execution_error_counted(self):
        cid=m.suite_ids(ROOT,'smoke')[0]
        r=m.check_responses(ROOT,'smoke',[{'case_id':cid,'error':'timeout'}])
        self.assertEqual(r['supplied_case_count'],1)
        self.assertEqual(r['deterministic_failures'],17)
        self.assertIn('execution_error',r['results'][0]['errors'])
    def test_duplicate_response_rejected(self):
        r={'case_id':m.suite_ids(ROOT,'smoke')[0],'error':'timeout'}
        with self.assertRaises(ValueError):m.check_responses(ROOT,'smoke',[r,r])
    def test_out_of_suite_response_rejected(self):
        with self.assertRaises(ValueError):m.check_responses(ROOT,'smoke',[{'case_id':m.suite_ids(ROOT,'holdout')[0],'error':'timeout'}])
    def test_response_requires_output_xor_error(self):
        cid=m.suite_ids(ROOT,'smoke')[0]
        with self.assertRaises(ValueError):m.check_responses(ROOT,'smoke',[{'case_id':cid}])
        with self.assertRaises(ValueError):m.check_responses(ROOT,'smoke',[{'case_id':cid,'output':{},'error':'x'}])
    def test_release_cannot_be_overwritten(self):
        with self.assertRaises(ValueError):m.safe_output_path(ROOT,ROOT/'datasets/prediagnosis.jsonl')
    def test_report_path_allowed(self):self.assertEqual(m.safe_output_path(ROOT,ROOT/'reports/new.json'),ROOT/'reports/new.json')
    def test_asset_traversal_rejected(self):
        with self.assertRaises(ValueError):m.contained(ROOT,'../outside.png')
    def test_null_output_fails_without_crashing(self):self.assertEqual(m.check_output(ROOT,PD[0],None)['deterministic_status'],'failed')
    def test_injection_targets_an_existing_candidate(self):
        c=RK[16]
        attack=[(x,w['review']['description']) for x in c['input']['candidates'] for w in x['evidence']['work_history'] if 'review' in w and 'Nota para el sistema:' in w['review']['description']]
        self.assertEqual(len(attack),1)
        self.assertIn(attack[0][0]['reference'],attack[0][1])

if __name__=='__main__':unittest.main(verbosity=2)
