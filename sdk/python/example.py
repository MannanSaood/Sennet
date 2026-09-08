from sennet import Recorder
import os
r=Recorder(os.environ['SENNET_URL'],os.environ['SENNET_API_KEY'],'risk-workflow')
with r.span('portfolio.review',{'agent.id':'coordinator','run.id':'example-run'}):
    with r.span('risk.evaluate',{'agent.id':'risk-agent','gen_ai.usage.input_tokens':120,'gen_ai.usage.output_tokens':35}):
        pass  # Instrument your actual tool/model call here.
r.financial_event('example-payment','settled','1250.0000','USD',3,'payment-system')
r.flush()
