import { useState } from 'react'
import './App.css'


function App() {
  const [response, setResponse] = useState()
  async function getAgents(){
    fetch("http://localhost:8080/agents").then((res)=>{return res.json()}).then((data)=>{console.log(data); setResponse(data)})
  }

  return (
    <>
      <button onClick={()=>{getAgents()}} className='cursor-pointer bg-blue-600 p-3 border-2 text-4xl font-bold'>
        get agents
      </button>
      <p>{JSON.stringify(response)}</p>
    </>
  )
}

export default App
