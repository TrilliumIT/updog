import React from 'react';
import ReactDOM from 'react-dom';
import axios from 'axios';
import './updog.css';

function Summary(props) {
	return(<div className="summary">{props.up}/{props.total} {props.mon_type}</div>)
}

class Updog extends React.Component {
	constructor() {
		super();
		this.state = {
			mon_types: ["application", "service", "instance"],
			data: undefined,
		}
	}

	componentDidMount() {
		axios.get('http://updog.dc0.cl.trilliumstaffing.com:8080/api/status/applications')
			.then(res => {
				this.setState({ data: res.data })
			});
	}

	render() {
		const data = Object.assign({}, this.state.data);

		const summaries = this.state.mon_types.map((mon_type) => {
			return (
				<Summary
					key={mon_type + '_summary'}
					mon_type={mon_type + 's'}
					up={data[mon_type + 's_up']}
					total={data[mon_type + 's_total']}
				/>
			);
		});

		const content = parseData(data, this.state.mon_types)

		return (
			<div>
			<div className="header">
				<div className="title">What's UpDog?</div>
				{summaries}
			</div>
			<div className="content">{content}</div>
			</div>
		);
	}
}

ReactDOM.render(<Updog />, document.getElementById('root'));

function parseData(data, mon_types) {
	return mon_types.map((mon_type) => {
		return Object.keys(data[mon_type + 's'] || {}).map((key) => {
			const d = parseData(data[mon_type + 's'][key], mon_types);
			return(
				<div className={mon_type}>
					<div className={mon_type+"_summary"}>
						<div className="title">{key}</div>
					</div>
					{d}
				</div>
			)
		});
	});
}
