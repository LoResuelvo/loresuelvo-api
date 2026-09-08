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
    def test_counts_and_origins(self):
        self.assertEqual((len(PD),len(RK)),(48,24))
        self.assertEqual(sum(c['origin']=='public_scenario_adaptation' for c in PD),16)
        self.assertTrue(all(c['review_status']=='finalized' and c['version']=='1.0.0' for c in PD+RK))
    def test_no_toy_shoe_or_spilled_glass(self):
        self.assertNotIn('zapatilla de tela',PD[3]['input']['user_message'])
        self.assertEqual(PD[5]['expected']['outcomes'],['collecting_information'])
    def test_spelling_pair_keeps_decision_and_facts(self):
        a,b=PD[6],PD[41]
        for key in ['outcomes','category_names','facts_to_preserve']:
            self.assertEqual(a['expected'][key],b['expected'][key])
    def test_equivalent_rank_topics_do_not_cross_split(self):
        for group in [[0,8,12,15,16,17,19,20,22,23],[1,9],[2,10,18],[3,11,13],[4,14,21]]:
            self.assertEqual(len({RK[i]['split'] for i in group}),1)
            self.assertEqual(len({RK[i]['family_id'] for i in group}),1)
    def test_real_source_is_not_a_label_source(self):
        for c in PD:
            self.assertTrue(set(c['provenance']['scenario_source_ids']).isdisjoint(c['provenance']['source_ids']))
    def test_reviews_have_variation(self):
        comments=[w['review']['description'] for c in RK for x in c['input']['candidates'] for w in x['evidence']['work_history'] if 'review' in w]
        self.assertGreater(len(set(comments)),40)
        self.assertNotIn('Se resolvió el problema descrito.',comments)
    def test_negative_reviews_do_not_claim_success(self):
        bad=[w['review']['description'] for x in RK[19]['input']['candidates'] for w in x['evidence']['work_history'] if 'review' in w and w['review']['rating']==1]
        self.assertEqual(len(bad),4)
        self.assertTrue(all(any(token in t.lower() for token in ['sigue','no quedó','no se arregló']) for t in bad))
    def test_collecting_valid_category_is_not_rejected(self):
        c=PD[16]
        out={'status':'answered','title':'Sin agua caliente','content':'¿Qué tipo de equipo tenés? ¿Notaste alguna señal de peligro?', 'image_descriptions':[],
             'assessment':{'action':'replace','outcome':'collecting_information','problem_title':'','problem_description':'','problem_category_name':'','selected_image_refs':[]}}
        result=m.check_output(ROOT,c,out)
        self.assertEqual(result['errors'],[])
        self.assertFalse(result['release_approved'])
        self.assertGreaterEqual(len(result['semantic_checks']),4)
        self.assertTrue(all(x['result']=='unassessed' and x['criterion'] for x in result['semantic_checks']))
    def test_collecting_fabricated_category_rejected(self):
        c=PD[16]
        out={'status':'answered','title':'Sin agua caliente','content':'Revisión.', 'image_descriptions':[],
             'assessment':{'action':'replace','outcome':'collecting_information','problem_title':'','problem_description':'','problem_category_name':'Gas','selected_image_refs':[]}}
        self.assertEqual(m.check_output(ROOT,c,out)['deterministic_status'],'failed')
    def test_export_contains_no_oracle_metadata(self):
        rows=m.export_inputs(ROOT,'development')
        self.assertEqual(len(rows),54)
        for r in rows:
            self.assertEqual(set(r),{'case_id','task','input'})
            self.assertFalse({'expected','provenance','split','family_id','risk','relevance','semantic_assertions'} & set(r['input']))
    def test_ranking_problem_titles_do_not_leak_oracle(self):
        forbidden=['empate','pertinente','igualmente','inyección','reseña','historial','evidencia','mérito','redacción','satisfacción']
        for c in RK:
            title=c['input']['problem_title'].lower()
            self.assertFalse(any(word in title for word in forbidden),c['id'])
    def test_injection_pair_preserves_base_message(self):
        self.assertTrue(PD[43]['input']['user_message'].startswith(PD[30]['input']['user_message']))
    def test_all_export_has_seventy_two(self):self.assertEqual(len(m.export_inputs(ROOT,'all')),72)
    def test_missing_responses_are_failures(self):
        r=m.check_responses(ROOT,'smoke',[])
        self.assertEqual(r['case_count'],18)
        self.assertEqual(r['deterministic_failures'],18)
        self.assertTrue(all('missing_response' in x['errors'] for x in r['results']))
    def test_explicit_execution_error_counted(self):
        cid=m.suite_ids(ROOT,'smoke')[0]
        r=m.check_responses(ROOT,'smoke',[{'case_id':cid,'error':'timeout'}])
        self.assertEqual(r['supplied_case_count'],1)
        self.assertEqual(r['deterministic_failures'],18)
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
        c=RK[20]
        attack=[(x,w['review']['description']) for x in c['input']['candidates'] for w in x['evidence']['work_history'] if 'review' in w and 'CANARIO_RK_60' in w['review']['description']]
        self.assertEqual(len(attack),1)
        self.assertIn(attack[0][0]['reference'],attack[0][1])

if __name__=='__main__':unittest.main(verbosity=2)
